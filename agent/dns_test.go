package agent

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	agentdns "github.com/hashicorp/consul/agent/dns"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

const (
	configUDPAnswerLimit   = 4
	defaultNumUDPResponses = 3
	testUDPTruncateLimit   = 8

	pctNodesWithIPv6 = 0.5

	// generateNumNodes is the upper bounds for the number of hosts used
	// in testing below.  Generate an arbitrarily large number of hosts.
	generateNumNodes = testUDPTruncateLimit * defaultNumUDPResponses * configUDPAnswerLimit
)

// makeRecursor creates a generic DNS server which always returns
// the provided reply. This is useful for mocking a DNS recursor with
// an expected result.
func makeRecursor(t *testing.T, answer dns.Msg) *dns.Server {
	a := answer
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(resp dns.ResponseWriter, msg *dns.Msg) {
		// The SetReply function sets the return code of the DNS
		// query to SUCCESS
		// We need a way to copy the variables not addressed
		// in SetReply
		answer.SetReply(msg)
		answer.Rcode = a.Rcode
		if err := resp.WriteMsg(&answer); err != nil {
			t.Fatalf("err: %s", err)
		}
	})
	up := make(chan struct{})
	server := &dns.Server{
		Addr:              "127.0.0.1:0",
		Net:               "udp",
		Handler:           mux,
		NotifyStartedFunc: func() { close(up) },
	}
	go server.ListenAndServe()
	<-up
	server.Addr = server.PacketConn.LocalAddr().String()
	return server
}

// dnsCNAME returns a DNS CNAME record struct
func dnsCNAME(src, dest string) *dns.CNAME {
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(src),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
		},
		Target: dns.Fqdn(dest),
	}
}

// dnsA returns a DNS A record struct
func dnsA(src, dest string) *dns.A {
	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(src),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
		},
		A: net.ParseIP(dest),
	}
}

// dnsTXT returns a DNS TXT record struct
func dnsTXT(src string, txt []string) *dns.TXT {
	return &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(src),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
		},
		Txt: txt,
	}
}

func TestRecursorAddr(t *testing.T) {
	t.Parallel()
	addr, err := recursorAddr("8.8.8.8")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if addr != "8.8.8.8:53" {
		t.Fatalf("bad: %v", addr)
	}
	addr, err = recursorAddr("2001:4860:4860::8888")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if addr != "[2001:4860:4860::8888]:53" {
		t.Fatalf("bad: %v", addr)
	}
	_, err = recursorAddr("1.2.3.4::53")
	if err == nil || !strings.Contains(err.Error(), "too many colons in address") {
		t.Fatalf("err: %v", err)
	}
	_, err = recursorAddr("2001:4860:4860::8888:::53")
	if err == nil || !strings.Contains(err.Error(), "too many colons in address") {
		t.Fatalf("err: %v", err)
	}
}

func TestEncodeKVasRFC1464(t *testing.T) {
	// Test cases are from rfc1464
	type rfc1464Test struct {
		key, value, internalForm, externalForm string
	}
	tests := []rfc1464Test{
		{"color", "blue", "color=blue", "color=blue"},
		{"equation", "a=4", "equation=a=4", "equation=a=4"},
		{"a=a", "true", "a`=a=true", "a`=a=true"},
		{"a\\=a", "false", "a\\`=a=false", "a\\`=a=false"},
		{"=", "\\=", "`==\\=", "`==\\="},

		{"string", "\"Cat\"", "string=\"Cat\"", "string=\"Cat\""},
		{"string2", "`abc`", "string2=``abc``", "string2=``abc``"},
		{"novalue", "", "novalue=", "novalue="},
		{"a b", "c d", "a b=c d", "a b=c d"},
		{"abc ", "123 ", "abc` =123 ", "abc` =123 "},

		// Additional tests
		{" abc", " 321", "` abc= 321", "` abc= 321"},
		{"`a", "b", "``a=b", "``a=b"},
	}

	for _, test := range tests {
		answer := encodeKVasRFC1464(test.key, test.value)
		require.Equal(t, test.internalForm, answer)
	}
}

func TestDNS_Over_TCP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "Foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.node.dc1.consul.", dns.TypeANY)

	c := new(dns.Client)
	c.Net = "tcp"
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("empty lookup: %#v", in)
	}
}

func TestDNS_EmptyAltDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("consul.service.", dns.TypeA)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Empty(t, in.Answer)
}

func TestDNS_NodeLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		TaggedAddresses: map[string]string{
			"wan": "127.0.0.2",
		},
		NodeMeta: map[string]string{
			"key": "value",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Answer, 2)
	require.Len(t, in.Extra, 0)

	aRec, ok := in.Answer[0].(*dns.A)
	require.True(t, ok, "First answer is not an A record")
	require.Equal(t, "127.0.0.1", aRec.A.String())
	require.Equal(t, uint32(0), aRec.Hdr.Ttl)

	txt, ok := in.Answer[1].(*dns.TXT)
	require.True(t, ok, "Second answer is not a TXT record")
	require.Len(t, txt.Txt, 1)
	require.Equal(t, "key=value", txt.Txt[0])

	// Re-do the query, but only for an A RR

	m = new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeA)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Answer, 1)
	require.Len(t, in.Extra, 1)

	aRec, ok = in.Answer[0].(*dns.A)
	require.True(t, ok, "Answer is not an A record")
	require.Equal(t, "127.0.0.1", aRec.A.String())
	require.Equal(t, uint32(0), aRec.Hdr.Ttl)

	txt, ok = in.Extra[0].(*dns.TXT)
	require.True(t, ok, "Extra record is not a TXT record")
	require.Len(t, txt.Txt, 1)
	require.Equal(t, "key=value", txt.Txt[0])

	// Re-do the query, but specify the DC
	m = new(dns.Msg)
	m.SetQuestion("foo.node.dc1.consul.", dns.TypeANY)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Answer, 2)
	require.Len(t, in.Extra, 0)

	aRec, ok = in.Answer[0].(*dns.A)
	require.True(t, ok, "First answer is not an A record")
	require.Equal(t, "127.0.0.1", aRec.A.String())
	require.Equal(t, uint32(0), aRec.Hdr.Ttl)

	_, ok = in.Answer[1].(*dns.TXT)
	require.True(t, ok, "Second answer is not a TXT record")

	// lookup a non-existing node, we should receive a SOA
	m = new(dns.Msg)
	m.SetQuestion("nofoo.node.dc1.consul.", dns.TypeANY)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Ns, 1)
	soaRec, ok := in.Ns[0].(*dns.SOA)
	require.True(t, ok, "NS RR is not a SOA record")
	require.Equal(t, uint32(0), soaRec.Hdr.Ttl)
}

func TestDNS_CaseInsensitiveNodeLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "Foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("fOO.node.dc1.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("empty lookup: %#v", in)
	}
}

func TestDNS_NodeLookup_PeriodName(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node with period in name
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo.bar",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.bar.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok := in.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_NodeLookup_AAAA(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "::4242:4242",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeAAAA)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok := in.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.AAAA.String() != "::4242:4242" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.Hdr.Ttl != 0 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNSCycleRecursorCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	// Start a DNS recursor that returns a SERVFAIL
	server1 := makeRecursor(t, dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeServerFailure},
	})
	// Start a DNS recursor that returns the result
	defer server1.Shutdown()
	server2 := makeRecursor(t, dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{
			dnsA("www.google.com", "172.21.45.67"),
		},
	})
	defer server2.Shutdown()
	// Mock the agent startup with the necessary configs
	agent := NewTestAgent(t,
		`recursors = ["`+server1.Addr+`", "`+server2.Addr+`"]
		`)
	defer agent.Shutdown()
	// DNS Message init
	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeA)
	// Agent request
	client := new(dns.Client)
	in, _, _ := client.Exchange(m, agent.DNSAddr())
	wantAnswer := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "www.google.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0xAC, 0x15, 0x2D, 0x43}, // 172 , 21, 45, 67
		},
	}
	require.Equal(t, wantAnswer, in.Answer)
}

func TestDNSCycleRecursorCheckAllFail(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	// Start 3 DNS recursors that returns a REFUSED status
	server1 := makeRecursor(t, dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeRefused},
	})
	defer server1.Shutdown()
	server2 := makeRecursor(t, dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeRefused},
	})
	defer server2.Shutdown()
	server3 := makeRecursor(t, dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeRefused},
	})
	defer server3.Shutdown()
	// Mock the agent startup with the necessary configs
	agent := NewTestAgent(t,
		`recursors = ["`+server1.Addr+`", "`+server2.Addr+`","`+server3.Addr+`"]
		`)
	defer agent.Shutdown()
	// DNS dummy message initialization
	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeA)
	// Agent request
	client := new(dns.Client)
	in, _, _ := client.Exchange(m, agent.DNSAddr())
	// Verify if we hit SERVFAIL from Consul
	require.Equal(t, dns.RcodeServerFailure, in.Rcode)
}

func TestDNS_NodeLookup_CNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
			dnsTXT("google.com", []string{"my_txt_value"}),
		},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "www.google.com",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeANY)
	m.SetEdns0(8192, true)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "google.node.consul.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 0, Rdlength: 0x10},
			Target: "www.google.com.",
		},
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "www.google.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Rdlength: 0x2},
			Target: "google.com.",
		},
		&dns.A{
			Hdr: dns.RR_Header{Name: "google.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x1, 0x2, 0x3, 0x4}, // 1.2.3.4
		},
		&dns.TXT{
			Hdr: dns.RR_Header{Name: "google.com.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Rdlength: 0xd},
			Txt: []string{"my_txt_value"},
		},
	}
	require.Equal(t, wantAnswer, in.Answer)
}

func TestDNS_NodeLookup_TXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"rfc1035-00": "value0",
			"key0":       "value1",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeTXT)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have the 1 TXT record reply
	if len(in.Answer) != 2 {
		t.Fatalf("Bad: %#v", in)
	}

	txtRec, ok := in.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if len(txtRec.Txt) != 1 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if txtRec.Txt[0] != "value0" && txtRec.Txt[0] != "key0=value1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_NodeLookup_TXT_DontSuppress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = false }`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"rfc1035-00": "value0",
			"key0":       "value1",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeTXT)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have the 1 TXT record reply
	if len(in.Answer) != 2 {
		t.Fatalf("Bad: %#v", in)
	}

	txtRec, ok := in.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if len(txtRec.Txt) != 1 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if txtRec.Txt[0] != "value0" && txtRec.Txt[0] != "key0=value1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_NodeLookup_ANY(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
		&dns.TXT{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Rdlength: 0xa},
			Txt: []string{"key=value"},
		},
	}
	require.Equal(t, wantAnswer, in.Answer)
}

func TestDNS_NodeLookup_ANY_DontSuppressTXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = false }`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
		&dns.TXT{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Rdlength: 0xa},
			Txt: []string{"key=value"},
		},
	}
	require.Equal(t, wantAnswer, in.Answer)
}

func TestDNS_NodeLookup_A_SuppressTXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = false }`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
	}

	var out struct{}
	require.NoError(t, a.RPC("Catalog.Register", args, &out))

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeA)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	wantAnswer := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
	}
	require.Equal(t, wantAnswer, in.Answer)

	// ensure TXT RR suppression
	require.Len(t, in.Extra, 0)
}

func TestDNS_EDNS0(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.2",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetEdns0(12345, true)
	m.SetQuestion("foo.node.dc1.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("empty lookup: %#v", in)
	}
	edns := in.IsEdns0()
	if edns == nil {
		t.Fatalf("empty edns: %#v", in)
	}
	if edns.UDPSize() != 12345 {
		t.Fatalf("bad edns size: %d", edns.UDPSize())
	}
}

func TestDNS_EDNS0_ECS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		require.NoError(t, a.RPC("Catalog.Register", args, &out))
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		require.NoError(t, a.RPC("PreparedQuery.Apply", args, &id))
	}

	cases := []struct {
		Name          string
		Question      string
		SubnetAddr    string
		SourceNetmask uint8
		ExpectedScope uint8
	}{
		{"global", "db.service.consul.", "198.18.0.1", 32, 0},
		{"query", "test.query.consul.", "198.18.0.1", 32, 32},
		{"query-subnet", "test.query.consul.", "198.18.0.0", 21, 21},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			c := new(dns.Client)
			// Query the service directly - should have a globally valid scope (0)
			m := new(dns.Msg)
			edns := new(dns.OPT)
			edns.Hdr.Name = "."
			edns.Hdr.Rrtype = dns.TypeOPT
			edns.SetUDPSize(12345)
			edns.SetDo(true)
			subnetOp := new(dns.EDNS0_SUBNET)
			subnetOp.Code = dns.EDNS0SUBNET
			subnetOp.Family = 1
			subnetOp.SourceNetmask = tc.SourceNetmask
			subnetOp.Address = net.ParseIP(tc.SubnetAddr)
			edns.Option = append(edns.Option, subnetOp)
			m.Extra = append(m.Extra, edns)
			m.SetQuestion(tc.Question, dns.TypeA)

			in, _, err := c.Exchange(m, a.DNSAddr())
			require.NoError(t, err)
			require.Len(t, in.Answer, 1)
			aRec, ok := in.Answer[0].(*dns.A)
			require.True(t, ok)
			require.Equal(t, "127.0.0.1", aRec.A.String())

			optRR := in.IsEdns0()
			require.NotNil(t, optRR)
			require.Len(t, optRR.Option, 1)

			subnet, ok := optRR.Option[0].(*dns.EDNS0_SUBNET)
			require.True(t, ok)
			require.Equal(t, uint16(1), subnet.Family)
			require.Equal(t, tc.SourceNetmask, subnet.SourceNetmask)
			require.Equal(t, tc.ExpectedScope, subnet.SourceScope)
			require.Equal(t, net.ParseIP(tc.SubnetAddr), subnet.Address)
		})
	}
}

func TestDNS_ReverseLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo2",
		Address:    "127.0.0.2",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("2.0.0.127.in-addr.arpa.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	ptrRec, ok := in.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if ptrRec.Ptr != "foo2.node.dc1.consul." {
		t.Fatalf("Bad: %#v", ptrRec)
	}
}

func TestDNS_ReverseLookup_CustomDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		domain = "custom"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo2",
		Address:    "127.0.0.2",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("2.0.0.127.in-addr.arpa.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	ptrRec, ok := in.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if ptrRec.Ptr != "foo2.node.dc1.custom." {
		t.Fatalf("Bad: %#v", ptrRec)
	}
}

func TestDNS_ReverseLookup_IPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "::4242:4242",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("2.4.2.4.2.4.2.4.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.ip6.arpa.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	ptrRec, ok := in.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if ptrRec.Ptr != "bar.node.dc1.consul." {
		t.Fatalf("Bad: %#v", ptrRec)
	}
}

func TestDNS_ServiceReverseLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
				Address: "127.0.0.2",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("2.0.0.127.in-addr.arpa.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	ptrRec, ok := in.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if ptrRec.Ptr != serviceCanonicalDNSName("db", "service", "dc1", "consul", nil)+"." {
		t.Fatalf("Bad: %#v", ptrRec)
	}
}

func TestDNS_ServiceReverseLookup_IPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "2001:db8::1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
				Address: "2001:db8::ff00:42:8329",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("9.2.3.8.2.4.0.0.0.0.f.f.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	ptrRec, ok := in.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if ptrRec.Ptr != serviceCanonicalDNSName("db", "service", "dc1", "consul", nil)+"." {
		t.Fatalf("Bad: %#v", ptrRec)
	}
}

func TestDNS_ServiceReverseLookup_CustomDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		domain = "custom"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
				Address: "127.0.0.2",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("2.0.0.127.in-addr.arpa.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	ptrRec, ok := in.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if ptrRec.Ptr != serviceCanonicalDNSName("db", "service", "dc1", "custom", nil)+"." {
		t.Fatalf("Bad: %#v", ptrRec)
	}
}

func TestDNS_SOA_Settings(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testSoaWithConfig := func(config string, ttl, expire, refresh, retry uint) {
		a := NewTestAgent(t, config)
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		// lookup a non-existing node, we should receive a SOA
		m := new(dns.Msg)
		m.SetQuestion("nofoo.node.dc1.consul.", dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		require.NoError(t, err)
		require.Len(t, in.Ns, 1)
		soaRec, ok := in.Ns[0].(*dns.SOA)
		require.True(t, ok, "NS RR is not a SOA record")
		require.Equal(t, uint32(ttl), soaRec.Minttl)
		require.Equal(t, uint32(expire), soaRec.Expire)
		require.Equal(t, uint32(refresh), soaRec.Refresh)
		require.Equal(t, uint32(retry), soaRec.Retry)
		require.Equal(t, uint32(ttl), soaRec.Hdr.Ttl)
	}
	// Default configuration
	testSoaWithConfig("", 0, 86400, 3600, 600)
	// Override all settings
	testSoaWithConfig("dns_config={soa={min_ttl=60,expire=43200,refresh=1800,retry=300}}", 60, 43200, 1800, 300)
	// Override partial settings
	testSoaWithConfig("dns_config={soa={min_ttl=60,expire=43200}}", 60, 43200, 3600, 600)
	// Override partial settings, part II
	testSoaWithConfig("dns_config={soa={refresh=1800,retry=300}}", 0, 86400, 1800, 300)
}

func TestDNS_ServiceReverseLookupNodeAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
				Address: "127.0.0.1",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("1.0.0.127.in-addr.arpa.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	ptrRec, ok := in.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if ptrRec.Ptr != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", ptrRec)
	}
}

func TestDNS_ServiceLookupNoMultiCNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "198.18.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "foo.node.consul",
			},
		}

		var out struct{}
		require.NoError(t, a.RPC("Catalog.Register", args, &out))
	}

	// Register a second node node with the same service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "198.18.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "bar.node.consul",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	// expect a CNAME and an A RR
	require.Len(t, in.Answer, 2)
	require.IsType(t, &dns.CNAME{}, in.Answer[0])
	require.IsType(t, &dns.A{}, in.Answer[1])
}

func TestDNS_ServiceLookupPreferNoCNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "198.18.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "198.18.0.1",
			},
		}

		var out struct{}
		require.NoError(t, a.RPC("Catalog.Register", args, &out))
	}

	// Register a second node node with the same service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "198.18.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "bar.node.consul",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	// expect a CNAME and an A RR
	require.Len(t, in.Answer, 1)
	aRec, ok := in.Answer[0].(*dns.A)
	require.Truef(t, ok, "Not an A RR")

	require.Equal(t, "db.service.consul.", aRec.Hdr.Name)
	require.Equal(t, "198.18.0.1", aRec.A.String())
}

func TestDNS_ServiceLookupMultiAddrNoCNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "198.18.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "198.18.0.1",
			},
		}

		var out struct{}
		require.NoError(t, a.RPC("Catalog.Register", args, &out))
	}

	// Register a second node node with the same service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "198.18.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "bar.node.consul",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register a second node node with the same service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "198.18.0.3",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "198.18.0.3",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	// expect a CNAME and an A RR
	require.Len(t, in.Answer, 2)
	require.IsType(t, &dns.A{}, in.Answer[0])
	require.IsType(t, &dns.A{}, in.Answer[1])
}

func TestDNS_ServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}

	// Lookup a non-existing service/query, we should receive an SOA.
	questions = []string{
		"nodb.service.consul.",
		"nope.query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Ns) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		soaRec, ok := in.Ns[0].(*dns.SOA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}
		if soaRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}

	}
}

func TestDNS_ServiceLookupWithInternalServiceAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		node_name = "my.test-node"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	// The service is using the consul DNS name as service address
	// which triggers a lookup loop and a subsequent stack overflow
	// crash.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Address: "db.service.consul",
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Looking up the service should not trigger a loop
	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.SRV{
			Hdr:      dns.RR_Header{Name: "db.service.consul.", Rrtype: 0x21, Class: 0x1, Rdlength: 0x1b},
			Priority: 0x1,
			Weight:   0x1,
			Port:     12345,
			Target:   "foo.node.dc1.consul.",
		},
	}
	require.Equal(t, wantAnswer, in.Answer, "answer")
	wantExtra := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "foo.node.dc1.consul.", Rrtype: 0x1, Class: 0x1, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
	}
	require.Equal(t, wantExtra, in.Extra, "extra")
}

func TestDNS_ConnectServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register
	{
		args := structs.TestRegisterRequestProxy(t)
		args.Address = "127.0.0.55"
		args.Service.Proxy.DestinationServiceName = "db"
		args.Service.Address = ""
		args.Service.Port = 12345
		var out struct{}
		require.Nil(t, a.RPC("Catalog.Register", args, &out))
	}

	// Look up the service
	questions := []string{
		"db.connect.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		require.Nil(t, err)
		require.Len(t, in.Answer, 1)

		srvRec, ok := in.Answer[0].(*dns.SRV)
		require.True(t, ok)
		require.Equal(t, uint16(12345), srvRec.Port)
		require.Equal(t, "foo.node.dc1.consul.", srvRec.Target)
		require.Equal(t, uint32(0), srvRec.Hdr.Ttl)

		cnameRec, ok := in.Extra[0].(*dns.A)
		require.True(t, ok)
		require.Equal(t, "foo.node.dc1.consul.", cnameRec.Hdr.Name)
		require.Equal(t, uint32(0), srvRec.Hdr.Ttl)
		require.Equal(t, "127.0.0.55", cnameRec.A.String())
	}
}

func TestDNS_VirtualIPLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := StartTestAgent(t, TestAgent{HCL: ``, Overrides: `peering = { test_allow_peer_registrations = true }`})
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	server, ok := a.delegate.(*consul.Server)
	require.True(t, ok)

	// The proxy service will not receive a virtual IP if the server is not assigning virtual IPs yet.
	retry.Run(t, func(r *retry.R) {
		_, entry, err := server.FSM().State().SystemMetadataGet(nil, structs.SystemMetadataVirtualIPsEnabled)
		require.NoError(r, err)
		require.NotNil(r, entry)
	})

	type testCase struct {
		name     string
		reg      *structs.RegisterRequest
		question string
		expect   string
	}

	run := func(t *testing.T, tc testCase) {
		var out struct{}
		require.Nil(t, a.RPC("Catalog.Register", tc.reg, &out))

		m := new(dns.Msg)
		m.SetQuestion(tc.question, dns.TypeA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		require.Nil(t, err)
		require.Len(t, in.Answer, 1)

		aRec, ok := in.Answer[0].(*dns.A)
		require.True(t, ok)
		require.Equal(t, tc.expect, aRec.A.String())
	}

	tt := []testCase{
		{
			name: "local query",
			reg: &structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.55",
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					Service: "web-proxy",
					Port:    12345,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "db",
					},
				},
			},
			question: "db.virtual.consul.",
			expect:   "240.0.0.1",
		},
		{
			name: "query for imported service",
			reg: &structs.RegisterRequest{
				PeerName:   "frontend",
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.55",
				Service: &structs.NodeService{
					PeerName: "frontend",
					Kind:     structs.ServiceKindConnectProxy,
					Service:  "web-proxy",
					Port:     12345,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "db",
					},
				},
			},
			question: "db.virtual.frontend.consul.",
			expect:   "240.0.0.2",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestDNS_IngressServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register ingress-gateway service
	{
		args := structs.TestRegisterIngressGateway(t)
		var out struct{}
		require.Nil(t, a.RPC("Catalog.Register", args, &out))
	}

	// Register db service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Address: "",
				Port:    80,
			},
		}

		var out struct{}
		require.Nil(t, a.RPC("Catalog.Register", args, &out))
	}

	// Register proxy-defaults with 'http' protocol
	{
		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		require.Nil(t, a.RPC("ConfigEntry.Apply", req, &out))
		require.True(t, out)
	}

	// Register ingress-gateway config entry
	{
		args := &structs.IngressGatewayConfigEntry{
			Name: "ingress-gateway",
			Kind: structs.IngressGateway,
			Listeners: []structs.IngressListener{
				{
					Port:     8888,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "db"},
						{Name: "api"},
					},
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var out bool
		require.Nil(t, a.RPC("ConfigEntry.Apply", req, &out))
		require.True(t, out)
	}

	// Look up the service
	questions := []string{
		"api.ingress.consul.",
		"api.ingress.dc1.consul.",
		"db.ingress.consul.",
		"db.ingress.dc1.consul.",
	}
	for _, question := range questions {
		t.Run(question, func(t *testing.T) {
			m := new(dns.Msg)
			m.SetQuestion(question, dns.TypeA)

			c := new(dns.Client)
			in, _, err := c.Exchange(m, a.DNSAddr())
			require.Nil(t, err)
			require.Len(t, in.Answer, 1)

			cnameRec, ok := in.Answer[0].(*dns.A)
			require.True(t, ok)
			require.Equal(t, question, cnameRec.Hdr.Name)
			require.Equal(t, uint32(0), cnameRec.Hdr.Ttl)
			require.Equal(t, "127.0.0.1", cnameRec.A.String())
		})
	}
}

func TestDNS_ExternalServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with an external service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "www.google.com",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service
	questions := []string{
		"db.service.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}
}

func TestDNS_InifiniteRecursion(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This test should not create an infinite recursion
	t.Parallel()
	a := NewTestAgent(t, `
		domain = "CONSUL."
		node_name = "test node"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the initial node with a service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "web",
			Address:    "web.service.consul.",
			Service: &structs.NodeService{
				Service: "web",
				Port:    12345,
				Address: "web.service.consul.",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly
	questions := []string{
		"web.service.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeA)
		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) < 1 {
			t.Fatalf("Bad: %#v", in)
		}
		aRec, ok := in.Answer[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if aRec.Target != "web.service.consul." {
			t.Fatalf("Bad: %#v, target:=%s", aRec, aRec.Target)
		}
	}
}

func TestDNS_ExternalServiceToConsulCNAMELookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		domain = "CONSUL."
		node_name = "test node"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the initial node with a service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "web",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "web",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an external service pointing to the 'web' service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "alias",
			Address:    "web.service.consul",
			Service: &structs.NodeService{
				Service: "alias",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly
	questions := []string{
		"alias.service.consul.",
		"alias.service.CoNsUl.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "web.service.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		if len(in.Extra) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "web.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

	}
}

func TestDNS_NSRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		domain = "CONSUL."
		node_name = "server1"
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("something.node.consul.", dns.TypeNS)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: "consul.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 0, Rdlength: 0x13},
			Ns:  "server1.node.dc1.consul.",
		},
	}
	require.Equal(t, wantAnswer, in.Answer, "answer")
	wantExtra := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "server1.node.dc1.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4, Ttl: 0},
			A:   net.ParseIP("127.0.0.1").To4(),
		},
	}

	require.Equal(t, wantExtra, in.Extra, "extra")
}

func TestDNS_AltDomain_NSRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		domain = "CONSUL."
		node_name = "server1"
		alt_domain = "test-domain."
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	questions := []struct {
		ask        string
		domain     string
		wantDomain string
	}{
		{"something.node.consul.", "consul.", "server1.node.dc1.consul."},
		{"something.node.test-domain.", "test-domain.", "server1.node.dc1.test-domain."},
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question.ask, dns.TypeNS)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		wantAnswer := []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{Name: question.domain, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 0, Rdlength: 0x13},
				Ns:  question.wantDomain,
			},
		}
		require.Equal(t, wantAnswer, in.Answer, "answer")
		wantExtra := []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{Name: question.wantDomain, Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4, Ttl: 0},
				A:   net.ParseIP("127.0.0.1").To4(),
			},
		}

		require.Equal(t, wantExtra, in.Extra, "extra")
	}

}

func TestDNS_NSRecords_IPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
 		domain = "CONSUL."
 		node_name = "server1"
 		advertise_addr = "::1"
 	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("server1.node.dc1.consul.", dns.TypeNS)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: "consul.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 0, Rdlength: 0x2},
			Ns:  "server1.node.dc1.consul.",
		},
	}
	require.Equal(t, wantAnswer, in.Answer, "answer")
	wantExtra := []dns.RR{
		&dns.AAAA{
			Hdr:  dns.RR_Header{Name: "server1.node.dc1.consul.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Rdlength: 0x10, Ttl: 0},
			AAAA: net.ParseIP("::1"),
		},
	}

	require.Equal(t, wantExtra, in.Extra, "extra")

}

func TestDNS_AltDomain_NSRecords_IPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		domain = "CONSUL."
		node_name = "server1"
		advertise_addr = "::1"
		alt_domain = "test-domain."
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	questions := []struct {
		ask        string
		domain     string
		wantDomain string
	}{
		{"server1.node.dc1.consul.", "consul.", "server1.node.dc1.consul."},
		{"server1.node.dc1.test-domain.", "test-domain.", "server1.node.dc1.test-domain."},
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question.ask, dns.TypeNS)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		wantAnswer := []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{Name: question.domain, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 0, Rdlength: 0x2},
				Ns:  question.wantDomain,
			},
		}
		require.Equal(t, wantAnswer, in.Answer, "answer")
		wantExtra := []dns.RR{
			&dns.AAAA{
				Hdr:  dns.RR_Header{Name: question.wantDomain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Rdlength: 0x10, Ttl: 0},
				AAAA: net.ParseIP("::1"),
			},
		}

		require.Equal(t, wantExtra, in.Extra, "extra")
	}

}

func TestDNS_ExternalServiceToConsulCNAMENestedLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		node_name = "test-node"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the initial node with a service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "web",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "web",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an external service pointing to the 'web' service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "alias",
			Address:    "web.service.consul",
			Service: &structs.NodeService{
				Service: "alias",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an external service pointing to the 'alias' service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "alias2",
			Address:    "alias.service.consul",
			Service: &structs.NodeService{
				Service: "alias2",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly
	questions := []string{
		"alias2.service.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "alias.service.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if len(in.Extra) != 2 {
			t.Fatalf("Bad: %#v", in)
		}

		cnameRec, ok := in.Extra[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Name != "alias.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Target != "web.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

		aRec, ok := in.Extra[1].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.Hdr.Name != "web.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddress_A(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "127.0.0.2",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "7f000002.addr.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "7f000002.addr.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.2" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_AltDomain_ServiceLookup_ServiceAddress_A(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		alt_domain = "test-domain"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "127.0.0.2",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []struct {
		ask        string
		wantDomain string
	}{
		{"db.service.consul.", "consul."},
		{id + ".query.consul.", "consul."},
		{"db.service.test-domain.", "test-domain."},
		{id + ".query.test-domain.", "test-domain."},
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question.ask, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "7f000002.addr.dc1."+question.wantDomain {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "7f000002.addr.dc1."+question.wantDomain {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.2" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddress_SRV(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
		},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		               recursors = ["`+recursor.Addr+`"]
		       `)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service whose address isn't an IP.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "www.google.com",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		// Should have google CNAME
		cnRec, ok := in.Extra[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnRec.Target != "google.com." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

		// Check we recursively resolve
		aRec, ok := in.Extra[1].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.A.String() != "1.2.3.4" {
			t.Fatalf("Bad: %s", aRec.A.String())
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddressIPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "2607:20:4005:808::200e",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "2607002040050808000000000000200e.addr.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.AAAA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "2607002040050808000000000000200e.addr.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.AAAA.String() != "2607:20:4005:808::200e" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_AltDomain_ServiceLookup_ServiceAddressIPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		alt_domain = "test-domain"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "2607:20:4005:808::200e",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []struct {
		ask  string
		want string
	}{
		{"db.service.consul.", "2607002040050808000000000000200e.addr.dc1.consul."},
		{"db.service.test-domain.", "2607002040050808000000000000200e.addr.dc1.test-domain."},
		{id + ".query.consul.", "2607002040050808000000000000200e.addr.dc1.consul."},
		{id + ".query.test-domain.", "2607002040050808000000000000200e.addr.dc1.test-domain."},
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question.ask, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != question.want {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.AAAA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != question.want {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.AAAA.String() != "2607:20:4005:808::200e" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_ServiceLookup_WanTranslation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, `
		datacenter = "dc1"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a1.Shutdown()

	a2 := NewTestAgent(t, `
		datacenter = "dc2"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a2.Shutdown()

	// Join WAN cluster
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	_, err := a2.JoinWAN([]string{addr})
	require.NoError(t, err)
	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.WANMembers(), 2)
		require.Len(r, a2.WANMembers(), 2)
	})

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc2",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		require.NoError(t, a2.RPC("PreparedQuery.Apply", args, &id))
	}

	type testCase struct {
		nodeTaggedAddresses    map[string]string
		serviceAddress         string
		serviceTaggedAddresses map[string]structs.ServiceAddress

		dnsAddr string

		expectedPort    uint16
		expectedAddress string
		expectedARRName string
	}

	cases := map[string]testCase{
		"node-addr-from-dc1": {
			dnsAddr:         a1.config.DNSAddrs[0].String(),
			expectedPort:    8080,
			expectedAddress: "127.0.0.1",
			expectedARRName: "foo.node.dc2.consul.",
		},
		"node-wan-from-dc1": {
			dnsAddr: a1.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			expectedPort:    8080,
			expectedAddress: "127.0.0.2",
			expectedARRName: "7f000002.addr.dc2.consul.",
		},
		"service-addr-from-dc1": {
			dnsAddr: a1.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			serviceAddress:  "10.0.1.1",
			expectedPort:    8080,
			expectedAddress: "10.0.1.1",
			expectedARRName: "0a000101.addr.dc2.consul.",
		},
		"service-wan-from-dc1": {
			dnsAddr: a1.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			serviceAddress: "10.0.1.1",
			serviceTaggedAddresses: map[string]structs.ServiceAddress{
				"wan": {
					Address: "198.18.0.1",
					Port:    80,
				},
			},
			expectedPort:    80,
			expectedAddress: "198.18.0.1",
			expectedARRName: "c6120001.addr.dc2.consul.",
		},
		"node-addr-from-dc2": {
			dnsAddr:         a2.config.DNSAddrs[0].String(),
			expectedPort:    8080,
			expectedAddress: "127.0.0.1",
			expectedARRName: "foo.node.dc2.consul.",
		},
		"node-wan-from-dc2": {
			dnsAddr: a2.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			expectedPort:    8080,
			expectedAddress: "127.0.0.1",
			expectedARRName: "foo.node.dc2.consul.",
		},
		"service-addr-from-dc2": {
			dnsAddr: a2.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			serviceAddress:  "10.0.1.1",
			expectedPort:    8080,
			expectedAddress: "10.0.1.1",
			expectedARRName: "0a000101.addr.dc2.consul.",
		},
		"service-wan-from-dc2": {
			dnsAddr: a2.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			serviceAddress: "10.0.1.1",
			serviceTaggedAddresses: map[string]structs.ServiceAddress{
				"wan": {
					Address: "198.18.0.1",
					Port:    80,
				},
			},
			expectedPort:    8080,
			expectedAddress: "10.0.1.1",
			expectedARRName: "0a000101.addr.dc2.consul.",
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			// Register a remote node with a service. This is in a retry since we
			// need the datacenter to have a route which takes a little more time
			// beyond the join, and we don't have direct access to the router here.
			retry.Run(t, func(r *retry.R) {
				args := &structs.RegisterRequest{
					Datacenter:      "dc2",
					Node:            "foo",
					Address:         "127.0.0.1",
					TaggedAddresses: tc.nodeTaggedAddresses,
					Service: &structs.NodeService{
						Service:         "db",
						Address:         tc.serviceAddress,
						Port:            8080,
						TaggedAddresses: tc.serviceTaggedAddresses,
					},
				}

				var out struct{}
				require.NoError(r, a2.RPC("Catalog.Register", args, &out))
			})

			// Look up the SRV record via service and prepared query.
			questions := []string{
				"db.service.dc2.consul.",
				id + ".query.dc2.consul.",
			}
			for _, question := range questions {
				m := new(dns.Msg)
				m.SetQuestion(question, dns.TypeSRV)

				c := new(dns.Client)

				addr := tc.dnsAddr
				in, _, err := c.Exchange(m, addr)
				require.NoError(t, err)
				require.Len(t, in.Answer, 1)
				srvRec, ok := in.Answer[0].(*dns.SRV)
				require.True(t, ok, "Bad: %#v", in.Answer[0])
				require.Equal(t, tc.expectedPort, srvRec.Port)

				aRec, ok := in.Extra[0].(*dns.A)
				require.True(t, ok, "Bad: %#v", in.Extra[0])
				require.Equal(t, tc.expectedARRName, aRec.Hdr.Name)
				require.Equal(t, tc.expectedAddress, aRec.A.String())
			}

			// Also check the A record directly
			for _, question := range questions {
				m := new(dns.Msg)
				m.SetQuestion(question, dns.TypeA)

				c := new(dns.Client)
				addr := tc.dnsAddr
				in, _, err := c.Exchange(m, addr)
				require.NoError(t, err)
				require.Len(t, in.Answer, 1)

				aRec, ok := in.Answer[0].(*dns.A)
				require.True(t, ok, "Bad: %#v", in.Answer[0])
				require.Equal(t, question, aRec.Hdr.Name)
				require.Equal(t, tc.expectedAddress, aRec.A.String())
			}
		})
	}
}

func TestDNS_Lookup_TaggedIPAddresses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		require.NoError(t, a.RPC("PreparedQuery.Apply", args, &id))
	}

	type testCase struct {
		nodeAddress            string
		nodeTaggedAddresses    map[string]string
		serviceAddress         string
		serviceTaggedAddresses map[string]structs.ServiceAddress

		expectedServiceIPv4Address string
		expectedServiceIPv6Address string
		expectedNodeIPv4Address    string
		expectedNodeIPv6Address    string
	}

	cases := map[string]testCase{
		"simple-ipv4": {
			serviceAddress: "127.0.0.2",
			nodeAddress:    "127.0.0.1",

			expectedServiceIPv4Address: "127.0.0.2",
			expectedServiceIPv6Address: "",
			expectedNodeIPv4Address:    "127.0.0.1",
			expectedNodeIPv6Address:    "",
		},
		"simple-ipv6": {
			serviceAddress: "::2",
			nodeAddress:    "::1",

			expectedServiceIPv6Address: "::2",
			expectedServiceIPv4Address: "",
			expectedNodeIPv6Address:    "::1",
			expectedNodeIPv4Address:    "",
		},
		"ipv4-with-tagged-ipv6": {
			serviceAddress: "127.0.0.2",
			nodeAddress:    "127.0.0.1",

			serviceTaggedAddresses: map[string]structs.ServiceAddress{
				structs.TaggedAddressLANIPv6: {Address: "::2"},
			},
			nodeTaggedAddresses: map[string]string{
				structs.TaggedAddressLANIPv6: "::1",
			},

			expectedServiceIPv4Address: "127.0.0.2",
			expectedServiceIPv6Address: "::2",
			expectedNodeIPv4Address:    "127.0.0.1",
			expectedNodeIPv6Address:    "::1",
		},
		"ipv6-with-tagged-ipv4": {
			serviceAddress: "::2",
			nodeAddress:    "::1",

			serviceTaggedAddresses: map[string]structs.ServiceAddress{
				structs.TaggedAddressLANIPv4: {Address: "127.0.0.2"},
			},
			nodeTaggedAddresses: map[string]string{
				structs.TaggedAddressLANIPv4: "127.0.0.1",
			},

			expectedServiceIPv4Address: "127.0.0.2",
			expectedServiceIPv6Address: "::2",
			expectedNodeIPv4Address:    "127.0.0.1",
			expectedNodeIPv6Address:    "::1",
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := &structs.RegisterRequest{
				Datacenter:      "dc1",
				Node:            "foo",
				Address:         tc.nodeAddress,
				TaggedAddresses: tc.nodeTaggedAddresses,
				Service: &structs.NodeService{
					Service:         "db",
					Address:         tc.serviceAddress,
					Port:            8080,
					TaggedAddresses: tc.serviceTaggedAddresses,
				},
			}

			var out struct{}
			require.NoError(t, a.RPC("Catalog.Register", args, &out))

			// Look up the SRV record via service and prepared query.
			questions := []string{
				"db.service.consul.",
				id + ".query.consul.",
			}
			for _, question := range questions {
				m := new(dns.Msg)
				m.SetQuestion(question, dns.TypeA)

				c := new(dns.Client)
				addr := a.config.DNSAddrs[0].String()
				in, _, err := c.Exchange(m, addr)
				require.NoError(t, err)

				if tc.expectedServiceIPv4Address != "" {
					require.Len(t, in.Answer, 1)
					aRec, ok := in.Answer[0].(*dns.A)
					require.True(t, ok, "Bad: %#v", in.Answer[0])
					require.Equal(t, question, aRec.Hdr.Name)
					require.Equal(t, tc.expectedServiceIPv4Address, aRec.A.String())
				} else {
					require.Len(t, in.Answer, 0)
				}

				m = new(dns.Msg)
				m.SetQuestion(question, dns.TypeAAAA)

				c = new(dns.Client)
				addr = a.config.DNSAddrs[0].String()
				in, _, err = c.Exchange(m, addr)
				require.NoError(t, err)

				if tc.expectedServiceIPv6Address != "" {
					require.Len(t, in.Answer, 1)
					aRec, ok := in.Answer[0].(*dns.AAAA)
					require.True(t, ok, "Bad: %#v", in.Answer[0])
					require.Equal(t, question, aRec.Hdr.Name)
					require.Equal(t, tc.expectedServiceIPv6Address, aRec.AAAA.String())
				} else {
					require.Len(t, in.Answer, 0)
				}
			}

			// Look up node
			m := new(dns.Msg)
			m.SetQuestion("foo.node.consul.", dns.TypeA)

			c := new(dns.Client)
			addr := a.config.DNSAddrs[0].String()
			in, _, err := c.Exchange(m, addr)
			require.NoError(t, err)

			if tc.expectedNodeIPv4Address != "" {
				require.Len(t, in.Answer, 1)
				aRec, ok := in.Answer[0].(*dns.A)
				require.True(t, ok, "Bad: %#v", in.Answer[0])
				require.Equal(t, "foo.node.consul.", aRec.Hdr.Name)
				require.Equal(t, tc.expectedNodeIPv4Address, aRec.A.String())
			} else {
				require.Len(t, in.Answer, 0)
			}

			m = new(dns.Msg)
			m.SetQuestion("foo.node.consul.", dns.TypeAAAA)

			c = new(dns.Client)
			addr = a.config.DNSAddrs[0].String()
			in, _, err = c.Exchange(m, addr)
			require.NoError(t, err)

			if tc.expectedNodeIPv6Address != "" {
				require.Len(t, in.Answer, 1)
				aRec, ok := in.Answer[0].(*dns.AAAA)
				require.True(t, ok, "Bad: %#v", in.Answer[0])
				require.Equal(t, "foo.node.consul.", aRec.Hdr.Name)
				require.Equal(t, tc.expectedNodeIPv6Address, aRec.AAAA.String())
			} else {
				require.Len(t, in.Answer, 0)
			}
		})
	}
}

func TestDNS_CaseInsensitiveServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	tests := []struct {
		name   string
		config string
	}{
		// UDP + EDNS
		{"normal", ""},
		{"cache", `dns_config{ allow_stale=true, max_stale="3h", use_cache=true, "cache_max_age"="3h"}`},
		{"cache-with-streaming", `
			rpc{
				enable_streaming=true
			}
			use_streaming_backend=true
			dns_config{ allow_stale=true, max_stale="3h", use_cache=true, "cache_max_age"="3h"}
		    `},
	}
	for _, tst := range tests {
		t.Run(fmt.Sprintf("A lookup %v", tst.name), func(t *testing.T) {
			a := NewTestAgent(t, tst.config)
			defer a.Shutdown()
			testrpc.WaitForLeader(t, a.RPC, "dc1")

			// Register a node with a service.
			{
				args := &structs.RegisterRequest{
					Datacenter: "dc1",
					Node:       "foo",
					Address:    "127.0.0.1",
					Service: &structs.NodeService{
						Service: "Db",
						Tags:    []string{"Primary"},
						Port:    12345,
					},
				}

				var out struct{}
				if err := a.RPC("Catalog.Register", args, &out); err != nil {
					t.Fatalf("err: %v", err)
				}
			}

			// Register an equivalent prepared query, as well as a name.
			var id string
			{
				args := &structs.PreparedQueryRequest{
					Datacenter: "dc1",
					Op:         structs.PreparedQueryCreate,
					Query: &structs.PreparedQuery{
						Name: "somequery",
						Service: structs.ServiceQuery{
							Service: "db",
						},
					},
				}
				if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
					t.Fatalf("err: %v", err)
				}
			}

			// Try some variations to make sure case doesn't matter.
			questions := []string{
				"primary.Db.service.consul.",
				"primary.db.service.consul.",
				"pRIMARY.dB.service.consul.",
				"PRIMARY.dB.service.consul.",
				"db.service.consul.",
				"DB.service.consul.",
				"Db.service.consul.",
				"somequery.query.consul.",
				"SomeQuery.query.consul.",
				"SOMEQUERY.query.consul.",
			}

			for _, question := range questions {
				m := new(dns.Msg)
				m.SetQuestion(question, dns.TypeSRV)

				c := new(dns.Client)
				retry.Run(t, func(r *retry.R) {
					in, _, err := c.Exchange(m, a.DNSAddr())
					if err != nil {
						r.Fatalf("err: %v", err)
					}

					if len(in.Answer) != 1 {
						r.Fatalf("question %v, empty lookup: %#v", question, in)
					}
				})
			}
		})
	}
}

func TestDNS_ServiceLookup_TagPeriod(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"v1.primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m1 := new(dns.Msg)
	m1.SetQuestion("v1.primary2.db.service.consul.", dns.TypeSRV)

	c1 := new(dns.Client)
	in, _, err := c1.Exchange(m1, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 0 {
		t.Fatalf("Bad: %#v", in)
	}

	m := new(dns.Msg)
	m.SetQuestion("v1.primary.db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err = c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	srvRec, ok := in.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if srvRec.Port != 12345 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Target != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", srvRec)
	}

	aRec, ok := in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Name != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
}

func TestDNS_PreparedQueryNearIPEDNS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ipCoord := lib.GenerateCoordinate(1 * time.Millisecond)
	serviceNodes := []struct {
		name    string
		address string
		coord   *coordinate.Coordinate
	}{
		{"foo1", "198.18.0.1", lib.GenerateCoordinate(1 * time.Millisecond)},
		{"foo2", "198.18.0.2", lib.GenerateCoordinate(10 * time.Millisecond)},
		{"foo3", "198.18.0.3", lib.GenerateCoordinate(30 * time.Millisecond)},
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	added := 0

	// Register nodes with a service
	for _, cfg := range serviceNodes {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       cfg.name,
			Address:    cfg.address,
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
			},
		}

		var out struct{}
		err := a.RPC("Catalog.Register", args, &out)
		require.NoError(t, err)

		// Send coordinate updates
		coordArgs := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       cfg.name,
			Coord:      cfg.coord,
		}
		err = a.RPC("Coordinate.Update", &coordArgs, &out)
		require.NoError(t, err)

		added += 1
	}

	fmt.Printf("Added %d service nodes\n", added)

	// Register a node without a service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "198.18.0.9",
		}

		var out struct{}
		err := a.RPC("Catalog.Register", args, &out)
		require.NoError(t, err)

		// Send coordinate updates for a few nodes.
		coordArgs := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Coord:      ipCoord,
		}
		err = a.RPC("Coordinate.Update", &coordArgs, &out)
		require.NoError(t, err)
	}

	// Register a prepared query Near = _ip
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "some.query.we.like",
				Service: structs.ServiceQuery{
					Service: "db",
					Near:    "_ip",
				},
			},
		}

		var id string
		err := a.RPC("PreparedQuery.Apply", args, &id)
		require.NoError(t, err)
	}
	retry.Run(t, func(r *retry.R) {
		m := new(dns.Msg)
		m.SetQuestion("some.query.we.like.query.consul.", dns.TypeA)
		m.SetEdns0(4096, false)
		o := new(dns.OPT)
		o.Hdr.Name = "."
		o.Hdr.Rrtype = dns.TypeOPT
		e := new(dns.EDNS0_SUBNET)
		e.Code = dns.EDNS0SUBNET
		e.Family = 1
		e.SourceNetmask = 32
		e.SourceScope = 0
		e.Address = net.ParseIP("198.18.0.9").To4()
		o.Option = append(o.Option, e)
		m.Extra = append(m.Extra, o)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			r.Fatalf("Error with call to dns.Client.Exchange: %s", err)
		}

		if len(serviceNodes) != len(in.Answer) {
			r.Fatalf("Expecting %d A RRs in response, Actual found was %d", len(serviceNodes), len(in.Answer))
		}

		for i, rr := range in.Answer {
			if aRec, ok := rr.(*dns.A); ok {
				if actual := aRec.A.String(); serviceNodes[i].address != actual {
					r.Fatalf("Expecting A RR #%d = %s, Actual RR was %s", i, serviceNodes[i].address, actual)
				}
			} else {
				r.Fatalf("DNS Answer contained a non-A RR")
			}
		}
	})
}

func TestDNS_PreparedQueryNearIP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ipCoord := lib.GenerateCoordinate(1 * time.Millisecond)
	serviceNodes := []struct {
		name    string
		address string
		coord   *coordinate.Coordinate
	}{
		{"foo1", "198.18.0.1", lib.GenerateCoordinate(1 * time.Millisecond)},
		{"foo2", "198.18.0.2", lib.GenerateCoordinate(10 * time.Millisecond)},
		{"foo3", "198.18.0.3", lib.GenerateCoordinate(30 * time.Millisecond)},
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	added := 0

	// Register nodes with a service
	for _, cfg := range serviceNodes {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       cfg.name,
			Address:    cfg.address,
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
			},
		}

		var out struct{}
		err := a.RPC("Catalog.Register", args, &out)
		require.NoError(t, err)

		// Send coordinate updates
		coordArgs := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       cfg.name,
			Coord:      cfg.coord,
		}
		err = a.RPC("Coordinate.Update", &coordArgs, &out)
		require.NoError(t, err)

		added += 1
	}

	fmt.Printf("Added %d service nodes\n", added)

	// Register a node without a service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "198.18.0.9",
		}

		var out struct{}
		err := a.RPC("Catalog.Register", args, &out)
		require.NoError(t, err)

		// Send coordinate updates for a few nodes.
		coordArgs := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Coord:      ipCoord,
		}
		err = a.RPC("Coordinate.Update", &coordArgs, &out)
		require.NoError(t, err)
	}

	// Register a prepared query Near = _ip
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "some.query.we.like",
				Service: structs.ServiceQuery{
					Service: "db",
					Near:    "_ip",
				},
			},
		}

		var id string
		err := a.RPC("PreparedQuery.Apply", args, &id)
		require.NoError(t, err)
	}

	retry.Run(t, func(r *retry.R) {
		m := new(dns.Msg)
		m.SetQuestion("some.query.we.like.query.consul.", dns.TypeA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			r.Fatalf("Error with call to dns.Client.Exchange: %s", err)
		}

		if len(serviceNodes) != len(in.Answer) {
			r.Fatalf("Expecting %d A RRs in response, Actual found was %d", len(serviceNodes), len(in.Answer))
		}

		for i, rr := range in.Answer {
			if aRec, ok := rr.(*dns.A); ok {
				if actual := aRec.A.String(); serviceNodes[i].address != actual {
					r.Fatalf("Expecting A RR #%d = %s, Actual RR was %s", i, serviceNodes[i].address, actual)
				}
			} else {
				r.Fatalf("DNS Answer contained a non-A RR")
			}
		}
	})
}

func TestDNS_ServiceLookup_PreparedQueryNamePeriod(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register a prepared query with a period in the name.
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "some.query.we.like",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}

		var id string
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("some.query.we.like.query.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	srvRec, ok := in.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if srvRec.Port != 12345 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Target != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", srvRec)
	}

	aRec, ok := in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Name != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
}

func TestDNS_ServiceLookup_Dedup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a single node with multiple instances of a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
				Tags:    []string{"replica"},
				Port:    12345,
			},
		}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db3",
				Service: "db",
				Tags:    []string{"replica"},
				Port:    12346,
			},
		}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query, make sure only
	// one IP is returned.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		aRec, ok := in.Answer[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}
}

func TestDNS_ServiceLookup_Dedup_SRV(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a single node with multiple instances of a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
				Tags:    []string{"replica"},
				Port:    12345,
			},
		}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db3",
				Service: "db",
				Tags:    []string{"replica"},
				Port:    12346,
			},
		}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query, make sure only
	// one IP is returned and two unique ports are returned.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 2 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 && srvRec.Port != 12346 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}

		srvRec, ok = in.Answer[1].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}
		if srvRec.Port != 12346 && srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Port == in.Answer[0].(*dns.SRV).Port {
			t.Fatalf("should be a different port")
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_Recurse(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{dnsA("apple.com", "1.2.3.4")},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) == 0 {
		t.Fatalf("Bad: %#v", in)
	}
	if in.Rcode != dns.RcodeSuccess {
		t.Fatalf("Bad: %#v", in)
	}
}

func TestDNS_Recurse_Truncation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	recursor := makeRecursor(t, dns.Msg{
		MsgHdr: dns.MsgHdr{Truncated: true},
		Answer: []dns.RR{dnsA("apple.com", "1.2.3.4")},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if in.Truncated != true {
		t.Fatalf("err: message should have been truncated %v", in)
	}
	if len(in.Answer) == 0 {
		t.Fatalf("Bad: Truncated message ignored, expected some reply %#v", in)
	}
	if in.Rcode != dns.RcodeSuccess {
		t.Fatalf("Bad: %#v", in)
	}
}

func TestDNS_RecursorTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	serverClientTimeout := 3 * time.Second
	testClientTimeout := serverClientTimeout + 5*time.Second

	resolverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}

	resolver, err := net.ListenUDP("udp", resolverAddr)
	if err != nil {
		t.Error(err)
	}
	defer resolver.Close()

	a := NewTestAgent(t, `
		recursors = ["`+resolver.LocalAddr().String()+`"] // host must cause a connection|read|write timeout
		dns_config {
			recursor_timeout = "`+serverClientTimeout.String()+`"
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	// This client calling the server under test must have a longer timeout than the one we set internally
	c := &dns.Client{Timeout: testClientTimeout}

	start := time.Now()
	in, _, err := c.Exchange(m, a.DNSAddr())

	duration := time.Since(start)

	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 0 {
		t.Fatalf("Bad: %#v", in)
	}
	if in.Rcode != dns.RcodeServerFailure {
		t.Fatalf("Bad: %#v", in)
	}

	if duration < serverClientTimeout {
		t.Fatalf("Expected the call to return after at least %f seconds but lasted only %f", serverClientTimeout.Seconds(), duration.Seconds())
	}

}

func TestDNS_ServiceLookup_FilterCritical(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register nodes with health checks in various states.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID: "serf",
				Name:    "serf",
				Status:  api.HealthCritical,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args2 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID: "serf",
				Name:    "serf",
				Status:  api.HealthCritical,
			},
		}
		if err := a.RPC("Catalog.Register", args2, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args3 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},
		}
		if err := a.RPC("Catalog.Register", args3, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args4 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}
		if err := a.RPC("Catalog.Register", args4, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args5 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "quux",
			Address:    "127.0.0.4",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthWarning,
			},
		}
		if err := a.RPC("Catalog.Register", args5, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Only 4 and 5 are not failing, so we should get 2 answers
		if len(in.Answer) != 2 {
			t.Fatalf("Bad: %#v", in)
		}

		ips := make(map[string]bool)
		for _, resp := range in.Answer {
			aRec := resp.(*dns.A)
			ips[aRec.A.String()] = true
		}

		if !ips["127.0.0.3"] {
			t.Fatalf("Bad: %#v should contain 127.0.0.3 (state healthy)", in)
		}
		if !ips["127.0.0.4"] {
			t.Fatalf("Bad: %#v should contain 127.0.0.4 (state warning)", in)
		}
	}
}

func TestDNS_ServiceLookup_OnlyFailing(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register nodes with all health checks in a critical state.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID: "serf",
				Name:    "serf",
				Status:  api.HealthCritical,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args2 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID: "serf",
				Name:    "serf",
				Status:  api.HealthCritical,
			},
		}
		if err := a.RPC("Catalog.Register", args2, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args3 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},
		}
		if err := a.RPC("Catalog.Register", args3, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// All 3 are failing, so we should get 0 answers and an NXDOMAIN response
		if len(in.Answer) != 0 {
			t.Fatalf("Bad: %#v", in)
		}

		if in.Rcode != dns.RcodeNameError {
			t.Fatalf("Bad: %#v", in)
		}
	}
}

func TestDNS_ServiceLookup_OnlyPassing(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		dns_config {
			only_passing = true
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register nodes with health checks in various states.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthPassing,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args2 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthWarning,
			},
		}

		if err := a.RPC("Catalog.Register", args2, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args3 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},
		}

		if err := a.RPC("Catalog.Register", args3, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service:     "db",
					OnlyPassing: true,
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Only 1 is passing, so we should only get 1 answer
		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		resp := in.Answer[0]
		aRec := resp.(*dns.A)

		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}

	newCfg := *a.Config
	newCfg.DNSOnlyPassing = false
	err := a.reloadConfigInternal(&newCfg)
	require.NoError(t, err)

	// only_passing is now false. we should now get two nodes
	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	require.Equal(t, 2, len(in.Answer))
	ips := []string{in.Answer[0].(*dns.A).A.String(), in.Answer[1].(*dns.A).A.String()}
	sort.Strings(ips)
	require.Equal(t, []string{"127.0.0.1", "127.0.0.2"}, ips)
}

func TestDNS_ServiceLookup_Randomize(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a large number of nodes.
	for i := 0; i < generateNumNodes; i++ {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    fmt.Sprintf("127.0.0.%d", i+1),
			Service: &structs.NodeService{
				Service: "web",
				Port:    8000,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "web",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query. Ensure the
	// response is randomized each time.
	questions := []string{
		"web.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		uniques := map[string]struct{}{}
		for i := 0; i < 10; i++ {
			m := new(dns.Msg)
			m.SetQuestion(question, dns.TypeANY)

			c := &dns.Client{Net: "udp"}
			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Response length should be truncated and we should get
			// an A record for each response.
			if len(in.Answer) != defaultNumUDPResponses {
				t.Fatalf("Bad: %#v", len(in.Answer))
			}

			// Collect all the names.
			var names []string
			for _, rec := range in.Answer {
				switch v := rec.(type) {
				case *dns.SRV:
					names = append(names, v.Target)
				case *dns.A:
					names = append(names, v.A.String())
				}
			}
			nameS := strings.Join(names, "|")

			// Tally the results.
			uniques[nameS] = struct{}{}
		}

		// Give some wiggle room. Since the responses are randomized and
		// there is a finite number of combinations, requiring 0
		// duplicates every test run eventually gives us failures.
		if len(uniques) < 2 {
			t.Fatalf("unique response ratio too low: %d/10\n%v", len(uniques), uniques)
		}
	}
}

func TestBinarySearch(t *testing.T) {
	t.Parallel()
	msgSrc := new(dns.Msg)
	msgSrc.Compress = true
	msgSrc.SetQuestion("redis.service.consul.", dns.TypeSRV)

	for i := 0; i < 5000; i++ {
		target := fmt.Sprintf("host-redis-%d-%d.test.acme.com.node.dc1.consul.", i/256, i%256)
		msgSrc.Answer = append(msgSrc.Answer, &dns.SRV{Hdr: dns.RR_Header{Name: "redis.service.consul.", Class: 1, Rrtype: dns.TypeSRV, Ttl: 0x3c}, Port: 0x4c57, Target: target})
		msgSrc.Extra = append(msgSrc.Extra, &dns.CNAME{Hdr: dns.RR_Header{Name: target, Class: 1, Rrtype: dns.TypeCNAME, Ttl: 0x3c}, Target: fmt.Sprintf("fx.168.%d.%d.", i/256, i%256)})
	}
	for _, compress := range []bool{true, false} {
		for idx, maxSize := range []int{12, 256, 512, 8192, 65535} {
			t.Run(fmt.Sprintf("binarySearch %d", maxSize), func(t *testing.T) {
				msg := new(dns.Msg)
				msgSrc.Compress = compress
				msgSrc.SetQuestion("redis.service.consul.", dns.TypeSRV)
				msg.Answer = msgSrc.Answer
				msg.Extra = msgSrc.Extra
				msg.Ns = msgSrc.Ns
				index := make(map[string]dns.RR, len(msg.Extra))
				indexRRs(msg.Extra, index)
				blen := dnsBinaryTruncate(msg, maxSize, index, true)
				msg.Answer = msg.Answer[:blen]
				syncExtra(index, msg)
				predicted := msg.Len()
				buf, err := msg.Pack()
				if err != nil {
					t.Error(err)
				}
				if predicted < len(buf) {
					t.Fatalf("Bug in DNS library: %d != %d", predicted, len(buf))
				}
				if len(buf) > maxSize || (idx != 0 && len(buf) < 16) {
					t.Fatalf("bad[%d]: %d > %d", idx, len(buf), maxSize)
				}
			})
		}
	}
}

func TestDNS_TCP_and_UDP_Truncate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		dns_config {
			enable_truncate = true
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	services := []string{"normal", "truncated"}
	for index, service := range services {
		numServices := (index * 5000) + 2
		for i := 1; i < numServices; i++ {
			args := &structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       fmt.Sprintf("%s-%d.acme.com", service, i),
				Address:    fmt.Sprintf("127.%d.%d.%d", 0, (i / 255), i%255),
				Service: &structs.NodeService{
					Service: service,
					Port:    8000,
				},
			}

			var out struct{}
			if err := a.RPC("Catalog.Register", args, &out); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Register an equivalent prepared query.
		var id string
		{
			args := &structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryCreate,
				Query: &structs.PreparedQuery{
					Name: service,
					Service: structs.ServiceQuery{
						Service: service,
					},
				},
			}
			if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Look up the service directly and via prepared query. Ensure the
		// response is truncated each time.
		questions := []string{
			fmt.Sprintf("%s.service.consul.", service),
			id + ".query.consul.",
		}
		protocols := []string{
			"tcp",
			"udp",
		}
		for _, maxSize := range []uint16{8192, 65535} {
			for _, qType := range []uint16{dns.TypeANY, dns.TypeA, dns.TypeSRV} {
				for _, question := range questions {
					for _, protocol := range protocols {
						for _, compress := range []bool{true, false} {
							t.Run(fmt.Sprintf("lookup %s %s (qType:=%d) compressed=%v", question, protocol, qType, compress), func(t *testing.T) {
								m := new(dns.Msg)
								m.SetQuestion(question, dns.TypeANY)
								maxSz := maxSize
								if protocol == "udp" {
									maxSz = 8192
								}
								m.SetEdns0(maxSz, true)
								c := new(dns.Client)
								c.Net = protocol
								m.Compress = compress
								in, _, err := c.Exchange(m, a.DNSAddr())
								if err != nil {
									t.Fatalf("err: %v", err)
								}
								// actually check if we need to have the truncate bit
								resbuf, err := in.Pack()
								if err != nil {
									t.Fatalf("Error while packing answer: %s", err)
								}
								if !in.Truncated && len(resbuf) > int(maxSz) {
									t.Fatalf("should have truncate bit %#v %#v", in, len(in.Answer))
								}
								// Check for the truncate bit
								buf, err := m.Pack()
								info := fmt.Sprintf("service %s question:=%s (%s) (%d total records) sz:= %d in %v",
									service, question, protocol, numServices, len(in.Answer), in)
								if err != nil {
									t.Fatalf("Error while packing: %v ; info:=%s", err, info)
								}
								if len(buf) > int(maxSz) {
									t.Fatalf("len(buf) := %d > maxSz=%d for %v", len(buf), maxSz, info)
								}
							})
						}
					}
				}
			}
		}
	}
}

func TestDNS_ServiceLookup_Truncate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		dns_config {
			enable_truncate = true
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a large number of nodes.
	for i := 0; i < generateNumNodes; i++ {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    fmt.Sprintf("127.0.0.%d", i+1),
			Service: &structs.NodeService{
				Service: "web",
				Port:    8000,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "web",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query. Ensure the
	// response is truncated each time.
	questions := []string{
		"web.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the truncate bit
		if !in.Truncated {
			t.Fatalf("should have truncate bit")
		}
	}
}

func TestDNS_ServiceLookup_LargeResponses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		dns_config {
			enable_truncate = true
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	longServiceName := "this-is-a-very-very-very-very-very-long-name-for-a-service"

	// Register a lot of nodes.
	for i := 0; i < 4; i++ {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    fmt.Sprintf("127.0.0.%d", i+1),
			Service: &structs.NodeService{
				Service: longServiceName,
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: longServiceName,
				Service: structs.ServiceQuery{
					Service: longServiceName,
					Tags:    []string{"primary"},
				},
			},
		}
		var id string
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"_" + longServiceName + "._primary.service.consul.",
		longServiceName + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if !in.Truncated {
			t.Fatalf("should have truncate bit")
		}

		// Make sure the response size is RFC 1035-compliant for UDP messages
		if in.Len() > 512 {
			t.Fatalf("Bad: %d", in.Len())
		}

		// We should only have two answers now
		if len(in.Answer) != 2 {
			t.Fatalf("Bad: %d", len(in.Answer))
		}

		// Make sure the ADDITIONAL section matches the ANSWER section.
		if len(in.Answer) != len(in.Extra) {
			t.Fatalf("Bad: %d vs. %d", len(in.Answer), len(in.Extra))
		}
		for i := 0; i < len(in.Answer); i++ {
			srv, ok := in.Answer[i].(*dns.SRV)
			if !ok {
				t.Fatalf("Bad: %#v", in.Answer[i])
			}

			a, ok := in.Extra[i].(*dns.A)
			if !ok {
				t.Fatalf("Bad: %#v", in.Extra[i])
			}

			if srv.Target != a.Hdr.Name {
				t.Fatalf("Bad: %#v %#v", srv, a)
			}
		}

		// Check for the truncate bit
		if !in.Truncated {
			t.Fatalf("should have truncate bit")
		}
	}
}

func testDNSServiceLookupResponseLimits(t *testing.T, answerLimit int, qType uint16,
	expectedService, expectedQuery, expectedQueryID int) (bool, error) {
	a := NewTestAgent(t, `
		node_name = "test-node"
		dns_config {
			udp_answer_limit = `+fmt.Sprintf("%d", answerLimit)+`
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	choices := perfectlyRandomChoices(generateNumNodes, pctNodesWithIPv6)
	for i := 0; i < generateNumNodes; i++ {
		nodeAddress := fmt.Sprintf("127.0.0.%d", i+1)
		if choices[i] {
			nodeAddress = fmt.Sprintf("fe80::%d", i+1)
		}
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    nodeAddress,
			Service: &structs.NodeService{
				Service: "api-tier",
				Port:    8080,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			return false, fmt.Errorf("err: %v", err)
		}
	}
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "api-tier",
				Service: structs.ServiceQuery{
					Service: "api-tier",
				},
			},
		}

		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			return false, fmt.Errorf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"api-tier.service.consul.",
		"api-tier.query.consul.",
		id + ".query.consul.",
	}
	for idx, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, qType)

		c := &dns.Client{Net: "udp"}
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			return false, fmt.Errorf("err: %v", err)
		}

		switch idx {
		case 0:
			if (expectedService > 0 && len(in.Answer) != expectedService) ||
				(expectedService < -1 && len(in.Answer) < lib.AbsInt(expectedService)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s, sz:=%d", len(in.Answer), answerLimit, qType, question, in.Len())
			}
		case 1:
			if (expectedQuery > 0 && len(in.Answer) != expectedQuery) ||
				(expectedQuery < -1 && len(in.Answer) < lib.AbsInt(expectedQuery)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s, sz:=%d", len(in.Answer), answerLimit, qType, question, in.Len())
			}
		case 2:
			if (expectedQueryID > 0 && len(in.Answer) != expectedQueryID) ||
				(expectedQueryID < -1 && len(in.Answer) < lib.AbsInt(expectedQueryID)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s, sz:=%d", len(in.Answer), answerLimit, qType, question, in.Len())
			}
		default:
			panic("abort")
		}
	}

	return true, nil
}

func checkDNSService(
	t *testing.T,
	generateNumNodes int,
	aRecordLimit int,
	qType uint16,
	expectedResultsCount int,
	udpSize uint16,
) {
	a := NewTestAgent(t, `
		node_name = "test-node"
		dns_config {
			a_record_limit = `+fmt.Sprintf("%d", aRecordLimit)+`
			udp_answer_limit = `+fmt.Sprintf("%d", aRecordLimit)+`
		}
	`)
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	choices := perfectlyRandomChoices(generateNumNodes, pctNodesWithIPv6)
	for i := 0; i < generateNumNodes; i++ {
		nodeAddress := fmt.Sprintf("127.0.0.%d", i+1)
		if choices[i] {
			nodeAddress = fmt.Sprintf("fe80::%d", i+1)
		}
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    nodeAddress,
			Service: &structs.NodeService{
				Service: "api-tier",
				Port:    8080,
			},
		}

		var out struct{}
		require.NoError(t, a.RPC("Catalog.Register", args, &out))
	}
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "api-tier",
				Service: structs.ServiceQuery{
					Service: "api-tier",
				},
			},
		}

		require.NoError(t, a.RPC("PreparedQuery.Apply", args, &id))
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"api-tier.service.consul.",
		"api-tier.query.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		question := question
		t.Run("question: "+question, func(t *testing.T) {

			m := new(dns.Msg)

			m.SetQuestion(question, qType)
			protocol := "tcp"
			if udpSize > 0 {
				protocol = "udp"
			}
			if udpSize > 512 {
				m.SetEdns0(udpSize, true)
			}
			c := &dns.Client{Net: protocol, UDPSize: 8192}
			in, _, err := c.Exchange(m, a.DNSAddr())
			require.NoError(t, err)

			t.Logf("DNS Response for %+v - %+v", m, in)

			require.Equal(t, expectedResultsCount, len(in.Answer),
				"%d/%d answers received for type %v for %s (%s)", len(in.Answer), expectedResultsCount, qType, question, protocol)
		})
	}
}

func TestDNS_ServiceLookup_ARecordLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	tests := []struct {
		name                   string
		aRecordLimit           int
		expectedAResults       int
		expectedAAAAResults    int
		expectedANYResults     int
		expectedSRVResults     int
		numNodesTotal          int
		udpSize                uint16
		_unused_udpAnswerLimit int // NOTE: this field is not used
	}{
		// UDP + EDNS
		{"udp-edns-1", 1, 1, 1, 1, 30, 30, 8192, 3},
		{"udp-edns-2", 2, 2, 2, 2, 30, 30, 8192, 3},
		{"udp-edns-3", 3, 3, 3, 3, 30, 30, 8192, 3},
		{"udp-edns-4", 4, 4, 4, 4, 30, 30, 8192, 3},
		{"udp-edns-5", 5, 5, 5, 5, 30, 30, 8192, 3},
		{"udp-edns-6", 6, 6, 6, 6, 30, 30, 8192, 3},
		{"udp-edns-max", 6, 2, 1, 3, 3, 3, 8192, 3},
		// All UDP without EDNS have a limit of 2 answers due to udpAnswerLimit
		// Even SRV records are limit to 2 records
		{"udp-limit-1", 1, 1, 0, 1, 1, 1, 512, 2},
		{"udp-limit-2", 2, 1, 1, 2, 2, 2, 512, 2},
		// AAAA results limited by size of payload
		{"udp-limit-3", 3, 1, 1, 2, 2, 2, 512, 2},
		{"udp-limit-4", 4, 1, 1, 2, 2, 2, 512, 2},
		{"udp-limit-5", 5, 1, 1, 2, 2, 2, 512, 2},
		{"udp-limit-6", 6, 1, 1, 2, 2, 2, 512, 2},
		{"udp-limit-max", 6, 1, 1, 2, 2, 2, 512, 2},
		// All UDP without EDNS and no udpAnswerLimit
		// Size of records is limited by UDP payload
		{"udp-1", 1, 1, 0, 1, 1, 1, 512, 0},
		{"udp-2", 2, 1, 1, 2, 2, 2, 512, 0},
		{"udp-3", 3, 1, 1, 2, 2, 2, 512, 0},
		{"udp-4", 4, 1, 1, 2, 2, 2, 512, 0},
		{"udp-5", 5, 1, 1, 2, 2, 2, 512, 0},
		{"udp-6", 6, 1, 1, 2, 2, 2, 512, 0},
		// Only 3 A and 3 SRV records on 512 bytes
		{"udp-max", 6, 1, 1, 2, 2, 2, 512, 0},

		{"tcp-1", 1, 1, 1, 1, 30, 30, 0, 0},
		{"tcp-2", 2, 2, 2, 2, 30, 30, 0, 0},
		{"tcp-3", 3, 3, 3, 3, 30, 30, 0, 0},
		{"tcp-4", 4, 4, 4, 4, 30, 30, 0, 0},
		{"tcp-5", 5, 5, 5, 5, 30, 30, 0, 0},
		{"tcp-6", 6, 6, 6, 6, 30, 30, 0, 0},
		{"tcp-max", 6, 1, 1, 2, 2, 2, 0, 0},
	}
	for _, test := range tests {
		test := test // capture loop var

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// All those queries should have at max queriesLimited elements

			t.Run("A", func(t *testing.T) {
				t.Parallel()
				checkDNSService(t, test.numNodesTotal, test.aRecordLimit, dns.TypeA, test.expectedAResults, test.udpSize)
			})

			t.Run("AAAA", func(t *testing.T) {
				t.Parallel()
				checkDNSService(t, test.numNodesTotal, test.aRecordLimit, dns.TypeAAAA, test.expectedAAAAResults, test.udpSize)
			})

			t.Run("ANY", func(t *testing.T) {
				t.Parallel()
				checkDNSService(t, test.numNodesTotal, test.aRecordLimit, dns.TypeANY, test.expectedANYResults, test.udpSize)
			})

			// No limits but the size of records for SRV records, since not subject to randomization issues
			t.Run("SRV", func(t *testing.T) {
				t.Parallel()
				checkDNSService(t, test.expectedSRVResults, test.aRecordLimit, dns.TypeSRV, test.numNodesTotal, test.udpSize)
			})
		})
	}
}

func TestDNS_ServiceLookup_AnswerLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	// Build a matrix of config parameters (udpAnswerLimit), and the
	// length of the response per query type and question.  Negative
	// values imply the test must return at least the abs(value) number
	// of records in the answer section.  This is required because, for
	// example, on OS-X and Linux, the number of answers returned in a
	// 512B response is different even though both platforms are x86_64
	// and using the same version of Go.
	//
	// TODO(sean@): Why is it not identical everywhere when using the
	// same compiler?
	tests := []struct {
		name                string
		udpAnswerLimit      int
		expectedAService    int
		expectedAQuery      int
		expectedAQueryID    int
		expectedAAAAService int
		expectedAAAAQuery   int
		expectedAAAAQueryID int
		expectedANYService  int
		expectedANYQuery    int
		expectedANYQueryID  int
	}{
		{"0", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{"1", 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{"2", 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
		{"3", 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
		{"4", 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
		{"5", 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		{"6", 6, 6, 6, 6, 6, 6, 5, 6, 6, -5},
		{"7", 7, 7, 7, 6, 7, 7, 5, 7, 7, -5},
		{"8", 8, 8, 8, 6, 8, 8, 5, 8, 8, -5},
		{"9", 9, 8, 8, 6, 8, 8, 5, 8, 8, -5},
		{"20", 20, 8, 8, 6, 8, 8, 5, 8, -5, -5},
		{"30", 30, 8, 8, 6, 8, 8, 5, 8, -5, -5},
	}
	for _, test := range tests {
		test := test // capture loop var
		t.Run(fmt.Sprintf("A lookup %v", test), func(t *testing.T) {
			t.Parallel()
			ok, err := testDNSServiceLookupResponseLimits(t, test.udpAnswerLimit, dns.TypeA, test.expectedAService, test.expectedAQuery, test.expectedAQueryID)
			if !ok {
				t.Fatalf("Expected service A lookup %s to pass: %v", test.name, err)
			}
		})

		t.Run(fmt.Sprintf("AAAA lookup %v", test), func(t *testing.T) {
			t.Parallel()
			ok, err := testDNSServiceLookupResponseLimits(t, test.udpAnswerLimit, dns.TypeAAAA, test.expectedAAAAService, test.expectedAAAAQuery, test.expectedAAAAQueryID)
			if !ok {
				t.Fatalf("Expected service AAAA lookup %s to pass: %v", test.name, err)
			}
		})

		t.Run(fmt.Sprintf("ANY lookup %v", test), func(t *testing.T) {
			t.Parallel()
			ok, err := testDNSServiceLookupResponseLimits(t, test.udpAnswerLimit, dns.TypeANY, test.expectedANYService, test.expectedANYQuery, test.expectedANYQueryID)
			if !ok {
				t.Fatalf("Expected service ANY lookup %s to pass: %v", test.name, err)
			}
		})
	}
}

func TestDNS_ServiceLookup_CNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
		},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a name for an address.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "google",
			Address:    "www.google.com",
			Service: &structs.NodeService{
				Service: "search",
				Port:    80,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "search",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"search.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Service CNAME, google CNAME, google A record
		if len(in.Answer) != 3 {
			t.Fatalf("Bad: %#v", in)
		}

		// Should have service CNAME
		cnRec, ok := in.Answer[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if cnRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		// Should have google CNAME
		cnRec, ok = in.Answer[1].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}
		if cnRec.Target != "google.com." {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}

		// Check we recursively resolve
		if _, ok := in.Answer[2].(*dns.A); !ok {
			t.Fatalf("Bad: %#v", in.Answer[2])
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddress_CNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
		},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a name for an address.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "google",
			Address:    "1.2.3.4",
			Service: &structs.NodeService{
				Service: "search",
				Port:    80,
				Address: "www.google.com",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "search",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"search.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Service CNAME, google CNAME, google A record
		if len(in.Answer) != 3 {
			t.Fatalf("Bad: %#v", in)
		}

		// Should have service CNAME
		cnRec, ok := in.Answer[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if cnRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		// Should have google CNAME
		cnRec, ok = in.Answer[1].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}
		if cnRec.Target != "google.com." {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}

		// Check we recursively resolve
		if _, ok := in.Answer[2].(*dns.A); !ok {
			t.Fatalf("Bad: %#v", in.Answer[2])
		}
	}
}

func TestDNS_NodeLookup_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
		},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
		dns_config {
			node_ttl = "10s"
			allow_stale = true
			max_stale = "1s"
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok := in.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	// Register node with IPv6
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "::4242:4242",
	}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check an IPv6 record
	m = new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeANY)

	in, _, err = c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aaaaRec, ok := in.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aaaaRec.AAAA.String() != "::4242:4242" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aaaaRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	// Register node with CNAME
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "www.google.com",
	}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m = new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeANY)

	in, _, err = c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have the CNAME record + a few A records
	if len(in.Answer) < 2 {
		t.Fatalf("Bad: %#v", in)
	}

	cnRec, ok := in.Answer[0].(*dns.CNAME)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if cnRec.Target != "www.google.com." {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if cnRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_ServiceLookup_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		dns_config {
			service_ttl = {
				"d*" = "42s"
				"db" = "10s"
				"db*" = "66s"
				"*" = "5s"
			}
		allow_stale = true
		max_stale = "1s"
		}
	`)
	defer a.Shutdown()

	for idx, service := range []string{"db", "dblb", "dk", "api"} {
		nodeName := fmt.Sprintf("foo%d", idx)
		address := fmt.Sprintf("127.0.0.%d", idx)
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       nodeName,
			Address:    address,
			Service: &structs.NodeService{
				Service: service,
				Tags:    []string{"primary"},
				Port:    12345 + idx,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	c := new(dns.Client)
	expectResult := func(dnsQuery string, expectedTTL uint32) {
		t.Run(dnsQuery, func(t *testing.T) {
			m := new(dns.Msg)
			m.SetQuestion(dnsQuery, dns.TypeSRV)

			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			if len(in.Answer) != 1 {
				t.Fatalf("Bad: %#v, len is %d", in, len(in.Answer))
			}

			srvRec, ok := in.Answer[0].(*dns.SRV)
			if !ok {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}
			if srvRec.Hdr.Ttl != expectedTTL {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}

			aRec, ok := in.Extra[0].(*dns.A)
			if !ok {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
			if aRec.Hdr.Ttl != expectedTTL {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
		})
	}
	// Should have its exact TTL
	expectResult("db.service.consul.", 10)
	// Should match db*
	expectResult("dblb.service.consul.", 66)
	// Should match d*
	expectResult("dk.service.consul.", 42)
	// Should match *
	expectResult("api.service.consul.", 5)
}

func TestDNS_PreparedQuery_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		dns_config {
			service_ttl = {
				"d*" = "42s"
				"db" = "10s"
				"db*" = "66s"
				"*" = "5s"
			}
		allow_stale = true
		max_stale = "1s"
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	for idx, service := range []string{"db", "dblb", "dk", "api"} {
		nodeName := fmt.Sprintf("foo%d", idx)
		address := fmt.Sprintf("127.0.0.%d", idx)
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       nodeName,
			Address:    address,
			Service: &structs.NodeService{
				Service: service,
				Tags:    []string{"primary"},
				Port:    12345 + idx,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		// Register prepared query without TTL and with TTL
		{
			args := &structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryCreate,
				Query: &structs.PreparedQuery{
					Name: service,
					Service: structs.ServiceQuery{
						Service: service,
					},
				},
			}

			var id string
			if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
				t.Fatalf("err: %v", err)
			}
			queryTTL := fmt.Sprintf("%s-ttl", service)
			args = &structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryCreate,
				Query: &structs.PreparedQuery{
					Name: queryTTL,
					Service: structs.ServiceQuery{
						Service: service,
					},
					DNS: structs.QueryDNSOptions{
						TTL: "18s",
					},
				},
			}

			if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
				t.Fatalf("err: %v", err)
			}
		}
	}

	c := new(dns.Client)
	expectResult := func(dnsQuery string, expectedTTL uint32) {
		t.Run(dnsQuery, func(t *testing.T) {
			m := new(dns.Msg)
			m.SetQuestion(dnsQuery, dns.TypeSRV)

			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			if len(in.Answer) != 1 {
				t.Fatalf("Bad: %#v, len is %d", in, len(in.Answer))
			}

			srvRec, ok := in.Answer[0].(*dns.SRV)
			if !ok {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}
			if srvRec.Hdr.Ttl != expectedTTL {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}

			aRec, ok := in.Extra[0].(*dns.A)
			if !ok {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
			if aRec.Hdr.Ttl != expectedTTL {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
		})
	}

	// Should have its exact TTL
	expectResult("db.query.consul.", 10)
	expectResult("db-ttl.query.consul.", 18)
	// Should match db*
	expectResult("dblb.query.consul.", 66)
	expectResult("dblb-ttl.query.consul.", 18)
	// Should match d*
	expectResult("dk.query.consul.", 42)
	expectResult("dk-ttl.query.consul.", 18)
	// Should be the default value
	expectResult("api.query.consul.", 5)
	expectResult("api-ttl.query.consul.", 18)
}

func TestDNS_PreparedQuery_Failover(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, `
		datacenter = "dc1"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a1.Shutdown()

	a2 := NewTestAgent(t, `
		datacenter = "dc2"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a2.Shutdown()

	// Join WAN cluster.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	if _, err := a2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	retry.Run(t, func(r *retry.R) {
		if got, want := len(a1.WANMembers()), 2; got < want {
			r.Fatalf("got %d WAN members want at least %d", got, want)
		}
		if got, want := len(a2.WANMembers()), 2; got < want {
			r.Fatalf("got %d WAN members want at least %d", got, want)
		}
	})

	// Register a remote node with a service. This is in a retry since we
	// need the datacenter to have a route which takes a little more time
	// beyond the join, and we don't have direct access to the router here.
	retry.Run(t, func(r *retry.R) {
		args := &structs.RegisterRequest{
			Datacenter: "dc2",
			Node:       "foo",
			Address:    "127.0.0.1",
			TaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			Service: &structs.NodeService{
				Service: "db",
			},
		}

		var out struct{}
		if err := a2.RPC("Catalog.Register", args, &out); err != nil {
			r.Fatalf("err: %v", err)
		}
	})

	// Register a local prepared query.
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "my-query",
				Service: structs.ServiceQuery{
					Service: "db",
					Failover: structs.QueryFailoverOptions{
						Datacenters: []string{"dc2"},
					},
				},
			},
		}
		var id string
		if err := a1.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the SRV record via the query.
	m := new(dns.Msg)
	m.SetQuestion("my-query.query.consul.", dns.TypeSRV)

	c := new(dns.Client)
	clAddr := a1.config.DNSAddrs[0]
	in, _, err := c.Exchange(m, clAddr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure we see the remote DC and that the address gets
	// translated.
	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}
	if in.Answer[0].Header().Name != "my-query.query.consul." {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	srv, ok := in.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if srv.Target != "7f000002.addr.dc2.consul." {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	a, ok := in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if a.Hdr.Name != "7f000002.addr.dc2.consul." {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if a.A.String() != "127.0.0.2" {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
}

func TestDNS_ServiceLookup_SRV_RFC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	questions := []string{
		"_db._primary.service.dc1.consul.",
		"_db._primary.service.consul.",
		"_db._primary.dc1.consul.",
		"_db._primary.consul.",
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}

}

func TestDNS_ServiceLookup_SRV_RFC_TCP_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	questions := []string{
		"_db._tcp.service.dc1.consul.",
		"_db._tcp.service.consul.",
		"_db._tcp.dc1.consul.",
		"_db._tcp.consul.",
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}

}

func TestDNS_ServiceLookup_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	tests := []struct {
		token   string
		results int
	}{
		{"root", 1},
		{"anonymous", 0},
	}
	for _, tt := range tests {
		t.Run("ACLToken == "+tt.token, func(t *testing.T) {
			a := NewTestAgent(t, `
				primary_datacenter = "dc1"

				acl {
					enabled = true
					default_policy = "deny"
					down_policy = "deny"

					tokens {
						initial_management = "root"
						default = "`+tt.token+`"
					}
				}
			`)
			defer a.Shutdown()
			testrpc.WaitForLeader(t, a.RPC, "dc1")

			// Register a service
			args := &structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.1",
				Service: &structs.NodeService{
					Service: "foo",
					Port:    12345,
				},
				WriteRequest: structs.WriteRequest{Token: "root"},
			}
			var out struct{}
			if err := a.RPC("Catalog.Register", args, &out); err != nil {
				t.Fatalf("err: %v", err)
			}

			// Set up the DNS query
			c := new(dns.Client)
			m := new(dns.Msg)
			m.SetQuestion("foo.service.consul.", dns.TypeA)

			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(in.Answer) != tt.results {
				t.Fatalf("Bad: %#v", in)
			}
		})
	}
}

func TestDNS_ServiceLookup_MetaTXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = true }`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAdditional := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.dc1.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
		&dns.TXT{
			Hdr: dns.RR_Header{Name: "bar.node.dc1.consul.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Rdlength: 0xa},
			Txt: []string{"key=value"},
		},
	}
	require.Equal(t, wantAdditional, in.Extra)
}

func TestDNS_ServiceLookup_SuppressTXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = false }`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAdditional := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.dc1.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
	}
	require.Equal(t, wantAdditional, in.Extra)
}

func TestDNS_AddressLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Look up the addresses
	cases := map[string]string{
		"7f000001.addr.dc1.consul.": "127.0.0.1",
	}
	for question, answer := range cases {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		require.Len(t, in.Answer, 1)

		require.Equal(t, dns.TypeA, in.Answer[0].Header().Rrtype)
		aRec, ok := in.Answer[0].(*dns.A)
		require.True(t, ok)
		require.Equal(t, aRec.A.To4().String(), answer)
		require.Zero(t, aRec.Hdr.Ttl)
	}
}

func TestDNS_AddressLookupANY(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Look up the addresses
	cases := map[string]string{
		"7f000001.addr.dc1.consul.": "127.0.0.1",
	}
	for question, answer := range cases {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())

		require.NoError(t, err)
		require.Len(t, in.Answer, 1)
		require.Equal(t, in.Answer[0].Header().Rrtype, dns.TypeA)
		aRec, ok := in.Answer[0].(*dns.A)
		require.True(t, ok)
		require.Equal(t, aRec.A.To4().String(), answer)
		require.Zero(t, aRec.Hdr.Ttl)

	}
}

func TestDNS_AddressLookupInvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Look up the addresses
	cases := map[string]string{
		"7f000001.addr.dc1.consul.": "",
	}
	for question := range cases {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		require.NoError(t, err)
		require.Zero(t, in.Rcode)
		require.Nil(t, in.Answer)
		require.NotNil(t, in.Extra)
		require.Len(t, in.Extra, 1)
	}
}

func TestDNS_AddressLookupIPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Look up the addresses
	cases := map[string]string{
		"2607002040050808000000000000200e.addr.consul.": "2607:20:4005:808::200e",
		"2607112040051808ffffffffffff200e.addr.consul.": "2607:1120:4005:1808:ffff:ffff:ffff:200e",
	}
	for question, answer := range cases {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeAAAA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		if in.Answer[0].Header().Rrtype != dns.TypeAAAA {
			t.Fatalf("Invalid type: %#v", in.Answer[0])
		}
		aaaaRec, ok := in.Answer[0].(*dns.AAAA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if aaaaRec.AAAA.To16().String() != answer {
			t.Fatalf("Bad: %#v", aaaaRec)
		}
		if aaaaRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}
}

func TestDNS_AddressLookupIPV6InvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Look up the addresses
	cases := map[string]string{
		"2607002040050808000000000000200e.addr.consul.": "2607:20:4005:808::200e",
		"2607112040051808ffffffffffff200e.addr.consul.": "2607:1120:4005:1808:ffff:ffff:ffff:200e",
	}
	for question := range cases {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if in.Answer != nil {
			t.Fatalf("Bad: %#v", in)
		}
	}
}

// TestDNS_NonExistentDC_Server verifies NXDOMAIN is returned when
// Consul server agent is queried for a service in a non-existent
// domain.
func TestDNS_NonExistentDC_Server(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("consul.service.dc2.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if in.Rcode != dns.RcodeNameError {
		t.Fatalf("Expected RCode: %#v, had: %#v", dns.RcodeNameError, in.Rcode)
	}
}

// TestDNS_NonExistentDC_RPC verifies NXDOMAIN is returned when
// Consul server agent is queried over RPC by a non-server agent
// for a service in a non-existent domain
func TestDNS_NonExistentDC_RPC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := NewTestAgent(t, `
		node_name = "test-server"
	`)

	defer s.Shutdown()
	c := NewTestAgent(t, `
		node_name = "test-client"
		bootstrap = false
		server = false
	`)
	defer c.Shutdown()

	// Join LAN cluster
	addr := fmt.Sprintf("127.0.0.1:%d", s.Config.SerfPortLAN)
	_, err := c.JoinLAN([]string{addr}, nil)
	require.NoError(t, err)
	testrpc.WaitForTestAgent(t, c.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("consul.service.dc2.consul.", dns.TypeANY)

	d := new(dns.Client)
	in, _, err := d.Exchange(m, c.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if in.Rcode != dns.RcodeNameError {
		t.Fatalf("Expected RCode: %#v, had: %#v", dns.RcodeNameError, in.Rcode)
	}
}

func TestDNS_NonExistingLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// lookup a non-existing node, we should receive a SOA
	m := new(dns.Msg)
	m.SetQuestion("nonexisting.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Ns) != 1 {
		t.Fatalf("Bad: %#v %#v", in, len(in.Answer))
	}

	soaRec, ok := in.Ns[0].(*dns.SOA)
	if !ok {
		t.Fatalf("Bad: %#v", in.Ns[0])
	}
	if soaRec.Hdr.Ttl != 0 {
		t.Fatalf("Bad: %#v", in.Ns[0])
	}
}

func TestDNS_NonExistingLookupEmptyAorAAAA(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a v6-only service and a v4-only service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foov6",
			Address:    "fe80::1",
			Service: &structs.NodeService{
				Service: "webv6",
				Port:    8000,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foov4",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "webv4",
				Port:    8000,
			},
		}

		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register equivalent prepared queries.
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "webv4",
				Service: structs.ServiceQuery{
					Service: "webv4",
				},
			},
		}

		var id string
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "webv6",
				Service: structs.ServiceQuery{
					Service: "webv6",
				},
			},
		}

		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Check for ipv6 records on ipv4-only service directly and via the
	// prepared query.
	questions := []string{
		"webv4.service.consul.",
		"webv4.query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeAAAA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		require.Len(t, in.Ns, 1)
		soaRec, ok := in.Ns[0].(*dns.SOA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}
		if soaRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}

		require.Equal(t, dns.RcodeSuccess, in.Rcode)
	}

	// Check for ipv4 records on ipv6-only service directly and via the
	// prepared query.
	questions = []string{
		"webv6.service.consul.",
		"webv6.query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Ns) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		soaRec, ok := in.Ns[0].(*dns.SOA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}
		if soaRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}

		if in.Rcode != dns.RcodeSuccess {
			t.Fatalf("Bad: %#v", in)
		}
	}
}

func TestDNS_AltDomains_Service(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		alt_domain = "test-domain."
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "test-node",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			NodeMeta: map[string]string{
				"key": "value",
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	questions := []struct {
		ask        string
		wantDomain string
	}{
		{"db.service.consul.", "test-node.node.dc1.consul."},
		{"db.service.test-domain.", "test-node.node.dc1.test-domain."},
		{"db.service.dc1.consul.", "test-node.node.dc1.consul."},
		{"db.service.dc1.test-domain.", "test-node.node.dc1.test-domain."},
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question.ask, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if got, want := srvRec.Target, question.wantDomain; got != want {
			t.Fatalf("SRV target invalid, got %v want %v", got, want)
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

		if got, want := aRec.Hdr.Name, question.wantDomain; got != want {
			t.Fatalf("A record header invalid, got %v want %v", got, want)
		}

		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

		txtRec, ok := in.Extra[1].(*dns.TXT)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if got, want := txtRec.Hdr.Name, question.wantDomain; got != want {
			t.Fatalf("TXT record header invalid, got %v want %v", got, want)
		}
		if txtRec.Txt[0] != "key=value" {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
	}
}

func TestDNS_AltDomains_SOA(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		node_name = "test-node"
		alt_domain = "test-domain."
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	questions := []struct {
		ask         string
		want_domain string
	}{
		{"test-node.node.consul.", "consul."},
		{"test-node.node.test-domain.", "test-domain."},
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question.ask, dns.TypeSOA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		soaRec, ok := in.Answer[0].(*dns.SOA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		if got, want := soaRec.Hdr.Name, question.want_domain; got != want {
			t.Fatalf("SOA name invalid, got %q want %q", got, want)
		}
		if got, want := soaRec.Ns, ("ns." + question.want_domain); got != want {
			t.Fatalf("SOA ns invalid, got %q want %q", got, want)
		}
	}
}

func TestDNS_AltDomains_Overlap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// this tests the domain matching logic in DNSServer when encountering more
	// than one potential match (i.e. ambiguous match)
	// it should select the longer matching domain when dispatching
	t.Parallel()
	a := NewTestAgent(t, `
		node_name = "test-node"
		alt_domain = "test.consul."
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	questions := []string{
		"test-node.node.consul.",
		"test-node.node.test.consul.",
		"test-node.node.dc1.consul.",
		"test-node.node.dc1.test.consul.",
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("failed to resolve ambiguous alt domain %q: %#v", question, in)
		}

		aRec, ok := in.Answer[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		if got, want := aRec.A.To4().String(), "127.0.0.1"; got != want {
			t.Fatalf("A ip invalid, got %v want %v", got, want)
		}
	}
}

func TestDNS_PreparedQuery_AllowStale(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		dns_config {
			allow_stale = true
			max_stale = "1s"
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := MockPreparedQuery{
		executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
			// Return a response that's perpetually too stale.
			reply.LastContact = 2 * time.Second
			return nil
		},
	}

	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure that the lookup terminates and results in an SOA since
	// the query doesn't exist.
	{
		m := new(dns.Msg)
		m.SetQuestion("nope.query.consul.", dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Ns) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		soaRec, ok := in.Ns[0].(*dns.SOA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}
		if soaRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}

	}
}

func TestDNS_InvalidQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Try invalid forms of queries that should hit the special invalid case
	// of our query parser.
	questions := []string{
		"consul.",
		"node.consul.",
		"service.consul.",
		"query.consul.",
		"foo.node.dc1.extra.more.consul.",
		"foo.service.dc1.extra.more.consul.",
		"foo.query.dc1.extra.more.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Ns) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		soaRec, ok := in.Ns[0].(*dns.SOA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}
		if soaRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}

	}
}

func TestDNS_PreparedQuery_AgentSource(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := MockPreparedQuery{
		executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
			// Check that the agent inserted its self-name and datacenter to
			// the RPC request body.
			if args.Agent.Datacenter != a.Config.Datacenter ||
				args.Agent.Node != a.Config.NodeName {
				t.Fatalf("bad: %#v", args.Agent)
			}
			return nil
		},
	}

	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	{
		m := new(dns.Msg)
		m.SetQuestion("foo.query.consul.", dns.TypeSRV)

		c := new(dns.Client)
		if _, _, err := c.Exchange(m, a.DNSAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestDNS_EDNS_Truncate_AgentSource(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		dns_config {
			enable_truncate = true
		}
	`)
	defer a.Shutdown()
	a.DNSDisableCompression(true)
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := MockPreparedQuery{
		executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
			// Check that the agent inserted its self-name and datacenter to
			// the RPC request body.
			if args.Agent.Datacenter != a.Config.Datacenter ||
				args.Agent.Node != a.Config.NodeName {
				t.Fatalf("bad: %#v", args.Agent)
			}
			for i := 0; i < 100; i++ {
				reply.Nodes = append(reply.Nodes, structs.CheckServiceNode{Node: &structs.Node{Node: "apple", Address: fmt.Sprintf("node.address:%d", i)}, Service: &structs.NodeService{Service: "appleService", Address: fmt.Sprintf("service.address:%d", i)}})
			}
			return nil
		},
	}

	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := new(dns.Msg)
	req.SetQuestion("foo.query.consul.", dns.TypeSRV)
	req.SetEdns0(2048, true)
	req.Compress = false

	c := new(dns.Client)
	resp, _, err := c.Exchange(req, a.DNSAddr())
	require.NoError(t, err)
	require.True(t, resp.Len() < 2048)
}

func TestDNS_trimUDPResponse_NoTrim(t *testing.T) {
	t.Parallel()
	req := &dns.Msg{}
	resp := &dns.Msg{
		Answer: []dns.RR{
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
		},
		Extra: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
		},
	}

	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" node_name = "dummy"`)
	if trimmed := trimUDPResponse(req, resp, cfg.DNSUDPAnswerLimit); trimmed {
		t.Fatalf("Bad %#v", *resp)
	}

	expected := &dns.Msg{
		Answer: []dns.RR{
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
		},
		Extra: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
		},
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

func TestDNS_trimUDPResponse_TrimLimit(t *testing.T) {
	t.Parallel()
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" node_name = "dummy"`)

	req, resp, expected := &dns.Msg{}, &dns.Msg{}, &dns.Msg{}
	for i := 0; i < cfg.DNSUDPAnswerLimit+1; i++ {
		target := fmt.Sprintf("ip-10-0-1-%d.node.dc1.consul.", 185+i)
		srv := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   "redis-cache-redis.service.consul.",
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
			},
			Target: target,
		}
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   target,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
			},
			A: net.ParseIP(fmt.Sprintf("10.0.1.%d", 185+i)),
		}

		resp.Answer = append(resp.Answer, srv)
		resp.Extra = append(resp.Extra, a)
		if i < cfg.DNSUDPAnswerLimit {
			expected.Answer = append(expected.Answer, srv)
			expected.Extra = append(expected.Extra, a)
		}
	}

	if trimmed := trimUDPResponse(req, resp, cfg.DNSUDPAnswerLimit); !trimmed {
		t.Fatalf("Bad %#v", *resp)
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

func TestDNS_trimUDPResponse_TrimLimitWithNS(t *testing.T) {
	t.Parallel()
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" node_name = "dummy"`)

	req, resp, expected := &dns.Msg{}, &dns.Msg{}, &dns.Msg{}
	for i := 0; i < cfg.DNSUDPAnswerLimit+1; i++ {
		target := fmt.Sprintf("ip-10-0-1-%d.node.dc1.consul.", 185+i)
		srv := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   "redis-cache-redis.service.consul.",
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
			},
			Target: target,
		}
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   target,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
			},
			A: net.ParseIP(fmt.Sprintf("10.0.1.%d", 185+i)),
		}
		ns := &dns.SOA{
			Hdr: dns.RR_Header{
				Name:   target,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
			},
			Ns: fmt.Sprintf("soa-%d", i),
		}

		resp.Answer = append(resp.Answer, srv)
		resp.Extra = append(resp.Extra, a)
		resp.Ns = append(resp.Ns, ns)
		if i < cfg.DNSUDPAnswerLimit {
			expected.Answer = append(expected.Answer, srv)
			expected.Extra = append(expected.Extra, a)
		}
	}

	if trimmed := trimUDPResponse(req, resp, cfg.DNSUDPAnswerLimit); !trimmed {
		t.Fatalf("Bad %#v", *resp)
	}
	require.LessOrEqual(t, resp.Len(), defaultMaxUDPSize)
	require.Len(t, resp.Ns, 0)
}

func TestDNS_trimTCPResponse_TrimLimitWithNS(t *testing.T) {
	t.Parallel()
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" node_name = "dummy"`)

	req, resp, expected := &dns.Msg{}, &dns.Msg{}, &dns.Msg{}
	for i := 0; i < 5000; i++ {
		target := fmt.Sprintf("ip-10-0-1-%d.node.dc1.consul.", 185+i)
		srv := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   "redis-cache-redis.service.consul.",
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
			},
			Target: target,
		}
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   target,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
			},
			A: net.ParseIP(fmt.Sprintf("10.0.1.%d", 185+i)),
		}
		ns := &dns.SOA{
			Hdr: dns.RR_Header{
				Name:   target,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
			},
			Ns: fmt.Sprintf("soa-%d", i),
		}

		resp.Answer = append(resp.Answer, srv)
		resp.Extra = append(resp.Extra, a)
		resp.Ns = append(resp.Ns, ns)
		if i < cfg.DNSUDPAnswerLimit {
			expected.Answer = append(expected.Answer, srv)
			expected.Extra = append(expected.Extra, a)
		}
	}
	req.Question = append(req.Question, dns.Question{Qtype: dns.TypeSRV})

	if trimmed := trimTCPResponse(req, resp); !trimmed {
		t.Fatalf("Bad %#v", *resp)
	}
	require.LessOrEqual(t, resp.Len(), 65523)
	require.Len(t, resp.Ns, 0)
}

func loadRuntimeConfig(t *testing.T, hcl string) *config.RuntimeConfig {
	t.Helper()
	result, err := config.Load(config.LoadOpts{HCL: []string{hcl}})
	require.NoError(t, err)
	require.Len(t, result.Warnings, 0)
	return result.RuntimeConfig
}

func TestDNS_trimUDPResponse_TrimSize(t *testing.T) {
	t.Parallel()
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" node_name = "dummy"`)

	req, resp := &dns.Msg{}, &dns.Msg{}
	for i := 0; i < 100; i++ {
		target := fmt.Sprintf("ip-10-0-1-%d.node.dc1.consul.", 185+i)
		srv := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   "redis-cache-redis.service.consul.",
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
			},
			Target: target,
		}
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   target,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
			},
			A: net.ParseIP(fmt.Sprintf("10.0.1.%d", 185+i)),
		}

		resp.Answer = append(resp.Answer, srv)
		resp.Extra = append(resp.Extra, a)
	}

	// We don't know the exact trim, but we know the resulting answer
	// data should match its extra data.
	if trimmed := trimUDPResponse(req, resp, cfg.DNSUDPAnswerLimit); !trimmed {
		t.Fatalf("Bad %#v", *resp)
	}
	if len(resp.Answer) == 0 || len(resp.Answer) != len(resp.Extra) {
		t.Fatalf("Bad %#v", *resp)
	}
	for i := range resp.Answer {
		srv, ok := resp.Answer[i].(*dns.SRV)
		if !ok {
			t.Fatalf("should be SRV")
		}

		a, ok := resp.Extra[i].(*dns.A)
		if !ok {
			t.Fatalf("should be A")
		}

		if srv.Target != a.Header().Name {
			t.Fatalf("Bad %#v vs. %#v", *srv, *a)
		}
	}
}

func TestDNS_trimUDPResponse_TrimSizeEDNS(t *testing.T) {
	t.Parallel()
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" node_name = "dummy"`)

	req, resp := &dns.Msg{}, &dns.Msg{}

	for i := 0; i < 100; i++ {
		target := fmt.Sprintf("ip-10-0-1-%d.node.dc1.consul.", 150+i)
		srv := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   "redis-cache-redis.service.consul.",
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
			},
			Target: target,
		}
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   target,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
			},
			A: net.ParseIP(fmt.Sprintf("10.0.1.%d", 150+i)),
		}

		resp.Answer = append(resp.Answer, srv)
		resp.Extra = append(resp.Extra, a)
	}

	// Copy over to a new slice since we are trimming both.
	reqEDNS, respEDNS := &dns.Msg{}, &dns.Msg{}
	reqEDNS.SetEdns0(2048, true)
	respEDNS.Answer = append(respEDNS.Answer, resp.Answer...)
	respEDNS.Extra = append(respEDNS.Extra, resp.Extra...)

	// Trim each response
	if trimmed := trimUDPResponse(req, resp, cfg.DNSUDPAnswerLimit); !trimmed {
		t.Errorf("expected response to be trimmed: %#v", resp)
	}
	if trimmed := trimUDPResponse(reqEDNS, respEDNS, cfg.DNSUDPAnswerLimit); !trimmed {
		t.Errorf("expected edns to be trimmed: %#v", resp)
	}

	// Check answer lengths
	if len(resp.Answer) == 0 || len(resp.Answer) != len(resp.Extra) {
		t.Errorf("bad response answer length: %#v", resp)
	}
	if len(respEDNS.Answer) == 0 || len(respEDNS.Answer) != len(respEDNS.Extra) {
		t.Errorf("bad edns answer length: %#v", resp)
	}

	// Due to the compression, we can't check exact equality of sizes, but we can
	// make two requests and ensure that the edns one returns a larger payload
	// than the non-edns0 one.
	if len(resp.Answer) >= len(respEDNS.Answer) {
		t.Errorf("expected edns have larger answer: %#v\n%#v", resp, respEDNS)
	}
	if len(resp.Extra) >= len(respEDNS.Extra) {
		t.Errorf("expected edns have larger extra: %#v\n%#v", resp, respEDNS)
	}

	// Verify that the things point where they should
	for i := range resp.Answer {
		srv, ok := resp.Answer[i].(*dns.SRV)
		if !ok {
			t.Errorf("%d should be an SRV", i)
		}

		a, ok := resp.Extra[i].(*dns.A)
		if !ok {
			t.Errorf("%d should be an A", i)
		}

		if srv.Target != a.Header().Name {
			t.Errorf("%d: bad %#v vs. %#v", i, srv, a)
		}
	}
}

func TestDNS_syncExtra(t *testing.T) {
	t.Parallel()
	resp := &dns.Msg{
		Answer: []dns.RR{
			// These two are on the same host so the redundant extra
			// records should get deduplicated.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1002,
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
			// This one isn't in the Consul domain so it will get a
			// CNAME and then an A record from the recursor.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1003,
				Target: "demo.consul.io.",
			},
			// This one isn't in the Consul domain and it will get
			// a CNAME and A record from a recursor that alters the
			// case of the name. This proves we look up in the index
			// in a case-insensitive way.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "insensitive.consul.io.",
			},
			// This is also a CNAME, but it'll be set up to loop to
			// make sure we don't crash.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "deadly.consul.io.",
			},
			// This is also a CNAME, but it won't have another record.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "nope.consul.io.",
			},
		},
		Extra: []dns.RR{
			// These should get deduplicated.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			// This is a normal CNAME followed by an A record but we
			// have flipped the order. The algorithm should emit them
			// in the opposite order.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "fakeserver.consul.io.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "demo.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "fakeserver.consul.io.",
			},
			// These differ in case to test case insensitivity.
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "INSENSITIVE.CONSUL.IO.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "Another.Server.Com.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "another.server.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			// This doesn't appear in the answer, so should get
			// dropped.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-186.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.186"),
			},
			// These two test edge cases with CNAME handling.
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "deadly.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "deadly.consul.io.",
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "nope.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "notthere.consul.io.",
			},
		},
	}

	index := make(map[string]dns.RR)
	indexRRs(resp.Extra, index)
	syncExtra(index, resp)

	expected := &dns.Msg{
		Answer: resp.Answer,
		Extra: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "demo.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "fakeserver.consul.io.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "fakeserver.consul.io.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "INSENSITIVE.CONSUL.IO.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "Another.Server.Com.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "another.server.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "deadly.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "deadly.consul.io.",
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "nope.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "notthere.consul.io.",
			},
		},
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

func TestDNS_Compression_trimUDPResponse(t *testing.T) {
	t.Parallel()
	cfg := loadRuntimeConfig(t, `data_dir = "a" bind_addr = "127.0.0.1" node_name = "dummy"`)

	req, m := dns.Msg{}, dns.Msg{}
	trimUDPResponse(&req, &m, cfg.DNSUDPAnswerLimit)
	if m.Compress {
		t.Fatalf("compression should be off")
	}

	// The trim function temporarily turns off compression, so we need to
	// make sure the setting gets restored properly.
	m.Compress = true
	trimUDPResponse(&req, &m, cfg.DNSUDPAnswerLimit)
	if !m.Compress {
		t.Fatalf("compression should be on")
	}
}

func TestDNS_Compression_Query(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		conn, err := dns.Dial("udp", a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Do a manual exchange with compression on (the default).
		a.DNSDisableCompression(false)
		if err := conn.WriteMsg(m); err != nil {
			t.Fatalf("err: %v", err)
		}
		p := make([]byte, dns.MaxMsgSize)
		compressed, err := conn.Read(p)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Disable compression and try again.
		a.DNSDisableCompression(true)
		if err := conn.WriteMsg(m); err != nil {
			t.Fatalf("err: %v", err)
		}
		unc, err := conn.Read(p)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// We can't see the compressed status given the DNS API, so we
		// just make sure the message is smaller to see if it's
		// respecting the flag.
		if compressed == 0 || unc == 0 || compressed >= unc {
			t.Fatalf("'%s' doesn't look compressed: %d vs. %d", question, compressed, unc)
		}
	}
}

func TestDNS_Compression_ReverseLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo2",
		Address:    "127.0.0.2",
	}
	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("2.0.0.127.in-addr.arpa.", dns.TypeANY)

	conn, err := dns.Dial("udp", a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do a manual exchange with compression on (the default).
	if err := conn.WriteMsg(m); err != nil {
		t.Fatalf("err: %v", err)
	}
	p := make([]byte, dns.MaxMsgSize)
	compressed, err := conn.Read(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Disable compression and try again.
	a.DNSDisableCompression(true)
	if err := conn.WriteMsg(m); err != nil {
		t.Fatalf("err: %v", err)
	}
	unc, err := conn.Read(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// We can't see the compressed status given the DNS API, so we just make
	// sure the message is smaller to see if it's respecting the flag.
	if compressed == 0 || unc == 0 || compressed >= unc {
		t.Fatalf("doesn't look compressed: %d vs. %d", compressed, unc)
	}
}

func TestDNS_Compression_Recurse(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{dnsA("apple.com", "1.2.3.4")},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	conn, err := dns.Dial("udp", a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do a manual exchange with compression on (the default).
	if err := conn.WriteMsg(m); err != nil {
		t.Fatalf("err: %v", err)
	}
	p := make([]byte, dns.MaxMsgSize)
	compressed, err := conn.Read(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Disable compression and try again.
	a.DNSDisableCompression(true)
	if err := conn.WriteMsg(m); err != nil {
		t.Fatalf("err: %v", err)
	}
	unc, err := conn.Read(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// We can't see the compressed status given the DNS API, so we just make
	// sure the message is smaller to see if it's respecting the flag.
	if compressed == 0 || unc == 0 || compressed >= unc {
		t.Fatalf("doesn't look compressed: %d vs. %d", compressed, unc)
	}
}

func TestDNSInvalidRegex(t *testing.T) {
	tests := []struct {
		desc    string
		in      string
		invalid bool
	}{
		{"Valid Hostname", "testnode", false},
		{"Valid Hostname", "test-node", false},
		{"Invalid Hostname with special chars", "test#$$!node", true},
		{"Invalid Hostname with special chars in the end", "testnode%^", true},
		{"Whitespace", "  ", true},
		{"Only special chars", "./$", true},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if got, want := agentdns.InvalidNameRe.MatchString(test.in), test.invalid; got != want {
				t.Fatalf("Expected %v to return %v", test.in, want)
			}
		})

	}
}

func TestDNS_ConfigReload(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, `
		recursors = ["8.8.8.8:53"]
		dns_config = {
			allow_stale = false
			max_stale = "20s"
			node_ttl = "10s"
			service_ttl = {
				"my_services*" = "5s"
				"my_specific_service" = "30s"
			}
			enable_truncate = false
			only_passing = false
			recursor_strategy = "sequential"
			recursor_timeout = "15s"
			disable_compression = false
			a_record_limit = 1
			enable_additional_node_meta_txt = false
			soa = {
				refresh = 1
				retry = 2
				expire = 3
				min_ttl = 4
			}
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	for _, s := range a.dnsServers {
		cfg := s.config.Load().(*dnsConfig)
		require.Equal(t, []string{"8.8.8.8:53"}, cfg.Recursors)
		require.Equal(t, agentdns.RecursorStrategy("sequential"), cfg.RecursorStrategy)
		require.False(t, cfg.AllowStale)
		require.Equal(t, 20*time.Second, cfg.MaxStale)
		require.Equal(t, 10*time.Second, cfg.NodeTTL)
		ttl, _ := cfg.GetTTLForService("my_services_1")
		require.Equal(t, 5*time.Second, ttl)
		ttl, _ = cfg.GetTTLForService("my_specific_service")
		require.Equal(t, 30*time.Second, ttl)
		require.False(t, cfg.EnableTruncate)
		require.False(t, cfg.OnlyPassing)
		require.Equal(t, 15*time.Second, cfg.RecursorTimeout)
		require.False(t, cfg.DisableCompression)
		require.Equal(t, 1, cfg.ARecordLimit)
		require.False(t, cfg.NodeMetaTXT)
		require.Equal(t, uint32(1), cfg.SOAConfig.Refresh)
		require.Equal(t, uint32(2), cfg.SOAConfig.Retry)
		require.Equal(t, uint32(3), cfg.SOAConfig.Expire)
		require.Equal(t, uint32(4), cfg.SOAConfig.Minttl)
	}

	newCfg := *a.Config
	newCfg.DNSRecursors = []string{"1.1.1.1:53"}
	newCfg.DNSAllowStale = true
	newCfg.DNSMaxStale = 21 * time.Second
	newCfg.DNSNodeTTL = 11 * time.Second
	newCfg.DNSServiceTTL = map[string]time.Duration{
		"2_my_services*":        6 * time.Second,
		"2_my_specific_service": 31 * time.Second,
	}
	newCfg.DNSEnableTruncate = true
	newCfg.DNSOnlyPassing = true
	newCfg.DNSRecursorStrategy = "random"
	newCfg.DNSRecursorTimeout = 16 * time.Second
	newCfg.DNSDisableCompression = true
	newCfg.DNSARecordLimit = 2
	newCfg.DNSNodeMetaTXT = true
	newCfg.DNSSOA.Refresh = 10
	newCfg.DNSSOA.Retry = 20
	newCfg.DNSSOA.Expire = 30
	newCfg.DNSSOA.Minttl = 40

	err := a.reloadConfigInternal(&newCfg)
	require.NoError(t, err)

	for _, s := range a.dnsServers {
		cfg := s.config.Load().(*dnsConfig)
		require.Equal(t, []string{"1.1.1.1:53"}, cfg.Recursors)
		require.Equal(t, agentdns.RecursorStrategy("random"), cfg.RecursorStrategy)
		require.True(t, cfg.AllowStale)
		require.Equal(t, 21*time.Second, cfg.MaxStale)
		require.Equal(t, 11*time.Second, cfg.NodeTTL)
		ttl, _ := cfg.GetTTLForService("my_services_1")
		require.Equal(t, time.Duration(0), ttl)
		ttl, _ = cfg.GetTTLForService("2_my_services_1")
		require.Equal(t, 6*time.Second, ttl)
		ttl, _ = cfg.GetTTLForService("my_specific_service")
		require.Equal(t, time.Duration(0), ttl)
		ttl, _ = cfg.GetTTLForService("2_my_specific_service")
		require.Equal(t, 31*time.Second, ttl)
		require.True(t, cfg.EnableTruncate)
		require.True(t, cfg.OnlyPassing)
		require.Equal(t, 16*time.Second, cfg.RecursorTimeout)
		require.True(t, cfg.DisableCompression)
		require.Equal(t, 2, cfg.ARecordLimit)
		require.True(t, cfg.NodeMetaTXT)
		require.Equal(t, uint32(10), cfg.SOAConfig.Refresh)
		require.Equal(t, uint32(20), cfg.SOAConfig.Retry)
		require.Equal(t, uint32(30), cfg.SOAConfig.Expire)
		require.Equal(t, uint32(40), cfg.SOAConfig.Minttl)
	}
}

func TestDNS_ReloadConfig_DuringQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	m := MockPreparedQuery{
		executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
			time.Sleep(100 * time.Millisecond)
			reply.Nodes = structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						ID:      "my_node",
						Address: "127.0.0.1",
					},
					Service: &structs.NodeService{
						Address: "127.0.0.1",
						Port:    8080,
					},
				},
			}
			return nil
		},
	}

	err := a.registerEndpoint("PreparedQuery", &m)
	require.NoError(t, err)

	{
		m := new(dns.Msg)
		m.SetQuestion("nope.query.consul.", dns.TypeA)

		timeout := time.NewTimer(time.Second)
		res := make(chan *dns.Msg)
		errs := make(chan error)

		go func() {
			c := new(dns.Client)
			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				errs <- err
				return
			}
			res <- in
		}()

		time.Sleep(50 * time.Millisecond)

		// reload the config halfway through, that should not affect the ongoing query
		newCfg := *a.Config
		newCfg.DNSAllowStale = true
		a.reloadConfigInternal(&newCfg)

		select {
		case in := <-res:
			require.Equal(t, "127.0.0.1", in.Answer[0].(*dns.A).A.String())
		case err := <-errs:
			require.NoError(t, err)
		case <-timeout.C:
			require.FailNow(t, "timeout")
		}
	}
}

func TestECSNotGlobalError(t *testing.T) {
	t.Run("wrap nil", func(t *testing.T) {
		e := ecsNotGlobalError{}
		require.True(t, errors.Is(e, errECSNotGlobal))
		require.False(t, errors.Is(e, fmt.Errorf("some other error")))
		require.Equal(t, nil, errors.Unwrap(e))
	})

	t.Run("wrap some error", func(t *testing.T) {
		e := ecsNotGlobalError{error: errNameNotFound}
		require.True(t, errors.Is(e, errECSNotGlobal))
		require.False(t, errors.Is(e, fmt.Errorf("some other error")))
		require.Equal(t, errNameNotFound, errors.Unwrap(e))
	})
}

// perfectlyRandomChoices assigns exactly the provided fraction of size items a
// true value, and then presents a random permutation of those boolean values.
func perfectlyRandomChoices(size int, frac float64) []bool {
	out := make([]bool, size)

	max := int(float64(size) * frac)
	for i := 0; i < max; i++ {
		out[i] = true
	}

	rand.Shuffle(size, func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

func TestPerfectlyRandomChoices(t *testing.T) {
	count := func(got []bool) int {
		var x int
		for _, v := range got {
			if v {
				x++
			}
		}
		return x
	}

	type testcase struct {
		size   int
		frac   float64
		expect int
	}

	run := func(t *testing.T, tc testcase) {
		got := perfectlyRandomChoices(tc.size, tc.frac)
		require.Equal(t, tc.expect, count(got))
	}

	cases := []testcase{
		// 100%
		{0, 1, 0},
		{1, 1, 1},
		{2, 1, 2},
		{3, 1, 3},
		{5, 1, 5},
		// 50%
		{0, 0.5, 0},
		{1, 0.5, 0},
		{2, 0.5, 1},
		{3, 0.5, 1},
		{5, 0.5, 2},
		// 10%
		{0, 0.1, 0},
		{1, 0.1, 0},
		{2, 0.1, 0},
		{3, 0.1, 0},
		{5, 0.1, 0},
		{10, 0.1, 1},
		{11, 0.1, 1},
		{15, 0.1, 1},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("size=%d frac=%g", tc.size, tc.frac), func(t *testing.T) {
			run(t, tc)
		})
	}
}
