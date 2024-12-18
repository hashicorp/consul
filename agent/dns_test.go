// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

/* Note: this file got to be 10k lines long and caused multiple IDE issues
 * as well as GitHub's UI unable to display diffs with large changes to this file.
 * This file has been broken up by moving:
 * - Node Lookup tests into dns_node_lookup_test.go
 * - Service Lookup tests into dn_service_lookup_test.go
 *
 * Please be aware of the size of each of these files and add tests / break
 * up tests accordingly.
 */
import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/hashicorp/serf/coordinate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/gossip/librtt"
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

// Copied to agent/dns/recursor_test.go
func TestDNS_RecursorAddr(t *testing.T) {
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

func TestDNS_EncodeKVasRFC1464(t *testing.T) {
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

func TestDNS_CycleRecursorCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	require.NotNil(t, in)
	require.Equal(t, wantAnswer, in.Answer)
}
func TestDNS_CycleRecursorCheckAllFail(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	in, _, err := client.Exchange(m, agent.DNSAddr())
	require.NoError(t, err)
	// Verify if we hit SERVFAIL from Consul
	require.NotNil(t, in)
	require.Equal(t, dns.RcodeServerFailure, in.Rcode)
}

func TestDNS_EDNS0(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
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
		require.NoError(t, a.RPC(context.Background(), "PreparedQuery.Apply", args, &id))
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

func TestDNS_SOA_Settings(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	testSoaWithConfig("dns_config={soa={min_ttl=60,expire=43200,refresh=1800,retry=300}} ", 60, 43200, 1800, 300)
	// Override partial settings
	testSoaWithConfig("dns_config={soa={min_ttl=60,expire=43200}} ", 60, 43200, 3600, 600)
	// Override partial settings, part II
	testSoaWithConfig("dns_config={soa={refresh=1800,retry=300}} ", 0, 86400, 1800, 300)
}

func TestDNS_VirtualIPLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := StartTestAgent(t, TestAgent{HCL: "", Overrides: `peering = { test_allow_peer_registrations = true } log_level = "debug"`})
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
		require.Nil(t, a.RPC(context.Background(), "Catalog.Register", tc.reg, &out))

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

func TestDNS_InifiniteRecursion(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This test should not create an infinite recursion
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
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

func TestDNS_NSRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

func TestDNS_Lookup_TaggedIPAddresses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		require.NoError(t, a.RPC(context.Background(), "PreparedQuery.Apply", args, &id))
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
			require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

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

func TestDNS_PreparedQueryNearIPEDNS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ipCoord := librtt.GenerateCoordinate(1 * time.Millisecond)
	serviceNodes := []struct {
		name    string
		address string
		coord   *coordinate.Coordinate
	}{
		{"foo1", "198.18.0.1", librtt.GenerateCoordinate(1 * time.Millisecond)},
		{"foo2", "198.18.0.2", librtt.GenerateCoordinate(10 * time.Millisecond)},
		{"foo3", "198.18.0.3", librtt.GenerateCoordinate(30 * time.Millisecond)},
	}

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
		err := a.RPC(context.Background(), "Catalog.Register", args, &out)
		require.NoError(t, err)

		// Send coordinate updates
		coordArgs := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       cfg.name,
			Coord:      cfg.coord,
		}
		err = a.RPC(context.Background(), "Coordinate.Update", &coordArgs, &out)
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
		err := a.RPC(context.Background(), "Catalog.Register", args, &out)
		require.NoError(t, err)

		// Send coordinate updates for a few nodes.
		coordArgs := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Coord:      ipCoord,
		}
		err = a.RPC(context.Background(), "Coordinate.Update", &coordArgs, &out)
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
		err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id)
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

	ipCoord := librtt.GenerateCoordinate(1 * time.Millisecond)
	serviceNodes := []struct {
		name    string
		address string
		coord   *coordinate.Coordinate
	}{
		{"foo1", "198.18.0.1", librtt.GenerateCoordinate(1 * time.Millisecond)},
		{"foo2", "198.18.0.2", librtt.GenerateCoordinate(10 * time.Millisecond)},
		{"foo3", "198.18.0.3", librtt.GenerateCoordinate(30 * time.Millisecond)},
	}

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
		err := a.RPC(context.Background(), "Catalog.Register", args, &out)
		require.NoError(t, err)

		// Send coordinate updates
		coordArgs := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       cfg.name,
			Coord:      cfg.coord,
		}
		err = a.RPC(context.Background(), "Coordinate.Update", &coordArgs, &out)
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
		err := a.RPC(context.Background(), "Catalog.Register", args, &out)
		require.NoError(t, err)

		// Send coordinate updates for a few nodes.
		coordArgs := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Coord:      ipCoord,
		}
		err = a.RPC(context.Background(), "Coordinate.Update", &coordArgs, &out)
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
		err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id)
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

func TestDNS_Recurse(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

// no way to run a v2 version of this test since it is calling a private function and not
// using a test agent.
func TestDNS_BinarySearch(t *testing.T) {
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
		var eg errgroup.Group
		for i := 1; i < numServices; i++ {
			j := i
			eg.Go(func() error {
				args := &structs.RegisterRequest{
					Datacenter: "dc1",
					Node:       fmt.Sprintf("%s-%d.acme.com", service, j),
					Address:    fmt.Sprintf("127.%d.%d.%d", 0, (j / 255), j%255),
					Service: &structs.NodeService{
						Service: service,
						Port:    8000,
					},
				}

				var out struct{}
				return a.RPC(context.Background(), "Catalog.Register", args, &out)
			})
		}
		if err := eg.Wait(); err != nil {
			t.Fatalf("error registering: %v", err)
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
			if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
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

func TestDNS_AddressLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		require.Nil(t, in.Ns)
		require.Nil(t, in.Extra)
	}
}

func TestDNS_AddressLookupANY(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		aRecord := in.Extra[0].(*dns.A)
		require.Equal(t, "7f000001.addr.dc1.consul.", aRecord.Hdr.Name)
		require.Equal(t, dns.TypeA, aRecord.Hdr.Rrtype)
		require.Zero(t, aRecord.Hdr.Ttl)
		require.Equal(t, "127.0.0.1", aRecord.A.String())
	}
}

func TestDNS_AddressLookupIPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	require.Equal(t, dns.RcodeNameError, in.Rcode)
	require.Equal(t, 0, len(in.Answer))
	require.Equal(t, 0, len(in.Extra))
	require.Equal(t, 1, len(in.Ns))
	soa := in.Ns[0].(*dns.SOA)
	require.Equal(t, "consul.", soa.Hdr.Name)
	require.Equal(t, "ns.consul.", soa.Ns)
	require.Equal(t, "hostmaster.consul.", soa.Mbox)
}

// TestDNS_NonExistentDC_RPC verifies NXDOMAIN is returned when
// Consul server agent is queried over RPC by a non-server agent
// for a service in a non-existent domain
func TestDNS_NonExistentDC_RPC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

func TestDNS_NonExistentLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

func TestDNS_NonExistentLookupEmptyAorAAAA(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
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

		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
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
		t.Run(question, func(t *testing.T) {
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
		})
	}

	// Check for ipv4 records on ipv6-only service directly and via the
	// prepared query.
	questions = []string{
		"webv6.service.consul.",
		"webv6.query.consul.",
	}
	for _, question := range questions {
		t.Run(question, func(t *testing.T) {
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
		})
	}
}

func TestDNS_AltDomains_Service(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

func TestDNS_AltDomain_DCName_Overlap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// this tests the DC name overlap with the consul domain/alt-domain
	// we should get response when DC suffix is a prefix of consul alt-domain
	a := NewTestAgent(t, `
		datacenter = "dc-test"
		node_name = "test-node"
		alt_domain = "test.consul."
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc-test")

	questions := []string{
		"test-node.node.dc-test.consul.",
		"test-node.node.dc-test.test.consul.",
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeA)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		require.Len(t, in.Answer, 1)

		aRec, ok := in.Answer[0].(*dns.A)
		require.True(t, ok)
		require.Equal(t, aRec.A.To4().String(), "127.0.0.1")
	}
}

func TestDNS_PreparedQuery_AllowStale(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1"  `)
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
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" `)

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
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" `)

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
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" `)

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
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1"  `)

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
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1" `)

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

func TestDNS_trimUDPResponse_TrimSizeMaxSize(t *testing.T) {
	cfg := loadRuntimeConfig(t, `node_name = "test" data_dir = "a" bind_addr = "127.0.0.1"             `)

	resp := &dns.Msg{}

	for i := 0; i < 600; i++ {
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

	reqEDNS, respEDNS := &dns.Msg{}, &dns.Msg{}
	reqEDNS.SetEdns0(math.MaxUint16, true)
	respEDNS.Answer = append(respEDNS.Answer, resp.Answer...)
	respEDNS.Extra = append(respEDNS.Extra, resp.Extra...)
	require.Greater(t, respEDNS.Len(), math.MaxUint16)
	t.Logf("length is: %v", respEDNS.Len())

	if trimmed := trimUDPResponse(reqEDNS, respEDNS, cfg.DNSUDPAnswerLimit); !trimmed {
		t.Errorf("expected edns to be trimmed: %#v", resp)
	}
	require.Greater(t, math.MaxUint16, respEDNS.Len())

	t.Logf("length is: %v", respEDNS.Len())

	if len(respEDNS.Answer) == 0 || len(respEDNS.Answer) != len(respEDNS.Extra) {
		t.Errorf("bad edns answer length: %#v", resp)
	}
}

func TestDNS_syncExtra(t *testing.T) {
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
	cfg := loadRuntimeConfig(t, `data_dir = "a" bind_addr = "127.0.0.1" node_name = "dummy" `)

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
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
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

func TestDNS_Compression_Recurse(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

// TestDNS_V1ConfigReload validates that the dns configuration is saved to the
// DNS server when v1 DNS is configured and reload config internal is called.
func TestDNS_V1ConfigReload(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		server, ok := s.(*DNSServer)
		require.True(t, ok)

		cfg := server.config.Load().(*dnsServerConfig)
		require.Equal(t, []string{"8.8.8.8:53"}, cfg.Recursors)
		require.Equal(t, structs.RecursorStrategy("sequential"), cfg.RecursorStrategy)
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
		server, ok := s.(*DNSServer)
		require.True(t, ok)

		cfg := server.config.Load().(*dnsServerConfig)
		require.Equal(t, []string{"1.1.1.1:53"}, cfg.Recursors)
		require.Equal(t, structs.RecursorStrategy("random"), cfg.RecursorStrategy)
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

func TestDNS_ECSNotGlobalError(t *testing.T) {
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

func TestDNS_PerfectlyRandomChoices(t *testing.T) {
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

type testCaseParseLocality struct {
	name                string
	labels              []string
	defaultEntMeta      acl.EnterpriseMeta
	enterpriseDNSConfig enterpriseDNSConfig
	expectedResult      queryLocality
	expectedOK          bool
}

func TestDNS_ParseLocality(t *testing.T) {
	testCases := getTestCasesParseLocality()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := &DNSServer{
				defaultEnterpriseMeta: tc.defaultEntMeta,
			}
			actualResult, actualOK := d.parseLocality(tc.labels,
				&dnsRequestConfig{
					dnsServerConfig: &dnsServerConfig{
						enterpriseDNSConfig: tc.enterpriseDNSConfig,
					},
					defaultEnterpriseMeta: tc.defaultEntMeta,
				})
			require.Equal(t, tc.expectedOK, actualOK)
			require.Equal(t, tc.expectedResult, actualResult)

		})
	}

}

func TestDNS_EffectiveDatacenter(t *testing.T) {
	type testCase struct {
		name          string
		queryLocality queryLocality
		defaultDC     string
		expected      string
	}
	testCases := []testCase{
		{
			name: "return datacenter first",
			queryLocality: queryLocality{
				datacenter:       "test-dc",
				peerOrDatacenter: "test-peer",
			},
			defaultDC: "default-dc",
			expected:  "test-dc",
		},
		{
			name: "return PeerOrDatacenter second",
			queryLocality: queryLocality{
				peerOrDatacenter: "test-peer",
			},
			defaultDC: "default-dc",
			expected:  "test-peer",
		},
		{
			name:          "return defaultDC as fallback",
			queryLocality: queryLocality{},
			defaultDC:     "default-dc",
			expected:      "default-dc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.queryLocality.effectiveDatacenter(tc.defaultDC)
			require.Equal(t, tc.expected, got)
		})
	}
}
