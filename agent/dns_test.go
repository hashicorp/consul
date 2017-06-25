package agent

import (
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/miekg/dns"
	"github.com/pascaldekloe/goe/verify"
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
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(resp dns.ResponseWriter, msg *dns.Msg) {
		answer.SetReply(msg)
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

func TestRecursorAddr(t *testing.T) {
	t.Parallel()
	addr, err := recursorAddr("8.8.8.8")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if addr != "8.8.8.8:53" {
		t.Fatalf("bad: %v", addr)
	}
}

func TestDNS_NodeLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		TaggedAddresses: map[string]string{
			"wan": "127.0.0.2",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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
	if aRec.Hdr.Ttl != 0 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	// Re-do the query, but specify the DC
	m = new(dns.Msg)
	m.SetQuestion("foo.node.dc1.consul.", dns.TypeANY)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok = in.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.Hdr.Ttl != 0 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	// lookup a non-existing node, we should receive a SOA
	m = new(dns.Msg)
	m.SetQuestion("nofoo.node.dc1.consul.", dns.TypeANY)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, addr.String())
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

func TestDNS_CaseInsensitiveNodeLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("empty lookup: %#v", in)
	}
}

func TestDNS_NodeLookup_PeriodName(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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

func TestDNS_NodeLookup_CNAME(t *testing.T) {
	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
		},
	})
	defer recursor.Shutdown()

	cfg := TestConfig()
	cfg.DNSRecursor = recursor.Addr
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

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

	c := new(dns.Client)
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have the service record, CNAME record + A record
	if len(in.Answer) != 3 {
		t.Fatalf("Bad: %#v", in)
	}

	cnRec, ok := in.Answer[0].(*dns.CNAME)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if cnRec.Target != "www.google.com." {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if cnRec.Hdr.Ttl != 0 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_EDNS0(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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

func TestDNS_ReverseLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	cfg := TestConfig()
	cfg.Domain = "custom"
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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

func TestDNS_ServiceLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.SRV{
			Hdr:      dns.RR_Header{Name: "db.service.consul.", Rrtype: 0x21, Class: 0x1, Rdlength: 0x15},
			Priority: 0x1,
			Weight:   0x1,
			Port:     12345,
			Target:   "foo.node.dc1.consul.",
		},
	}
	verify.Values(t, "answer", in.Answer, wantAnswer)

	wantExtra := []dns.RR{
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "foo.node.dc1.consul.", Rrtype: 0x5, Class: 0x1, Rdlength: 0x2},
			Target: "db.service.consul.",
		},
		&dns.A{
			Hdr: dns.RR_Header{Name: "db.service.consul.", Rrtype: 0x1, Class: 0x1, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
	}
	verify.Values(t, "extra", in.Extra, wantExtra)
}

func TestDNS_ExternalServiceLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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

		cnameRec, ok := in.Extra[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_ExternalServiceToConsulCNAMELookup(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.Domain = "CONSUL."
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
		if srvRec.Target != "alias.node.dc1.consul." {
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
		if cnameRec.Hdr.Name != "alias.node.dc1.consul." {
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

func TestDNS_ExternalServiceToConsulCNAMENestedLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
		if srvRec.Target != "alias2.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		if len(in.Extra) != 3 {
			t.Fatalf("Bad: %#v", in)
		}

		cnameRec, ok := in.Extra[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Name != "alias2.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Target != "alias.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

		cnameRec, ok = in.Extra[1].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if cnameRec.Hdr.Name != "alias.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if cnameRec.Target != "web.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if cnameRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}

		aRec, ok := in.Extra[2].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[2])
		}
		if aRec.Hdr.Name != "web.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[2])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[2])
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddress_A(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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

func TestDNS_ServiceLookup_ServiceAddress_CNAME(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a node with a service whose address isn't an IP.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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

		cnameRec, ok := in.Extra[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddressIPV6(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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

func TestDNS_ServiceLookup_WanAddress(t *testing.T) {
	t.Parallel()
	cfg1 := TestConfig()
	cfg1.Datacenter = "dc1"
	cfg1.TranslateWanAddrs = true
	cfg1.ACLDatacenter = ""
	a1 := NewTestAgent(t.Name(), cfg1)
	defer a1.Shutdown()

	cfg2 := TestConfig()
	cfg2.Datacenter = "dc2"
	cfg2.TranslateWanAddrs = true
	cfg2.ACLDatacenter = ""
	a2 := NewTestAgent(t.Name(), cfg2)
	defer a2.Shutdown()

	// Join WAN cluster
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.Ports.SerfWan)
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
		if err := a2.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the SRV record via service and prepared query.
	questions := []string{
		"db.service.dc2.consul.",
		id + ".query.dc2.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a1.Config.ClientListener("", a1.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "7f000002.addr.dc2.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.2" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}

	// Also check the A record directly
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeA)

		c := new(dns.Client)
		addr, _ := a1.Config.ClientListener("", a1.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
		if aRec.Hdr.Name != question {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if aRec.A.String() != "127.0.0.2" {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}

	// Now query from the same DC and make sure we get the local address
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a2.Config.ClientListener("", a2.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "foo.node.dc2.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}

	// Also check the A record directly from DC2
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeA)

		c := new(dns.Client)
		addr, _ := a2.Config.ClientListener("", a2.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
		if aRec.Hdr.Name != question {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}
}

func TestDNS_CaseInsensitiveServiceLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "Db",
				Tags:    []string{"Master"},
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
		"master.db.service.consul.",
		"mASTER.dB.service.consul.",
		"MASTER.dB.service.consul.",
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("empty lookup: %#v", in)
		}
	}
}

func TestDNS_ServiceLookup_TagPeriod(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"v1.master"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("v1.master.db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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

func TestDNS_ServiceLookup_PreparedQueryNamePeriod(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a single node with multiple instances of a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
				Tags:    []string{"slave"},
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
				Tags:    []string{"slave"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a single node with multiple instances of a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
				Tags:    []string{"slave"},
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
				Tags:    []string{"slave"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{dnsA("apple.com", "1.2.3.4")},
	})
	defer recursor.Shutdown()

	cfg := TestConfig()
	cfg.DNSRecursor = recursor.Addr
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()

	recursor := makeRecursor(t, dns.Msg{
		MsgHdr: dns.MsgHdr{Truncated: true},
		Answer: []dns.RR{dnsA("apple.com", "1.2.3.4")},
	})
	defer recursor.Shutdown()

	cfg := TestConfig()
	cfg.DNSRecursor = recursor.Addr
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
	if err != dns.ErrTruncated {
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

	cfg := TestConfig()
	cfg.DNSRecursor = resolver.LocalAddr().String() // host must cause a connection|read|write timeout
	cfg.DNSConfig.RecursorTimeout = serverClientTimeout
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	// This client calling the server under test must have a longer timeout than the one we set internally
	c := &dns.Client{Timeout: testClientTimeout}
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)

	start := time.Now()
	in, _, err := c.Exchange(m, addr.String())

	duration := time.Now().Sub(start)

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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register nodes with health checks in various states.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
				Tags:    []string{"master"},
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
				Tags:    []string{"master"},
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
				Tags:    []string{"master"},
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
				Tags:    []string{"master"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register nodes with all health checks in a critical state.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
				Tags:    []string{"master"},
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
				Tags:    []string{"master"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	cfg := TestConfig()
	cfg.DNSConfig.OnlyPassing = true
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	// Register nodes with health checks in various states.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
				Tags:    []string{"master"},
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
				Tags:    []string{"master"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
}

func TestDNS_ServiceLookup_Randomize(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		for i := 0; i < 10; i++ {
			m := new(dns.Msg)
			m.SetQuestion(question, dns.TypeANY)

			c := &dns.Client{Net: "udp"}
			in, _, err := c.Exchange(m, addr.String())
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

func TestDNS_ServiceLookup_Truncate(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.DNSConfig.EnableTruncate = true
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

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

		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		c := new(dns.Client)
		in, _, err := c.Exchange(m, addr.String())
		if err != nil && err != dns.ErrTruncated {
			t.Fatalf("err: %v", err)
		}

		// Check for the truncate bit
		if !in.Truncated {
			t.Fatalf("should have truncate bit")
		}
	}
}

func TestDNS_ServiceLookup_LargeResponses(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.DNSConfig.EnableTruncate = true
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	longServiceName := "this-is-a-very-very-very-very-very-long-name-for-a-service"

	// Register a lot of nodes.
	for i := 0; i < 4; i++ {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    fmt.Sprintf("127.0.0.%d", i+1),
			Service: &structs.NodeService{
				Service: longServiceName,
				Tags:    []string{"master"},
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
					Tags:    []string{"master"},
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
		"_" + longServiceName + "._master.service.consul.",
		longServiceName + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
		if err != nil && err != dns.ErrTruncated {
			t.Fatalf("err: %v", err)
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

func testDNS_ServiceLookup_responseLimits(t *testing.T, answerLimit int, qType uint16,
	expectedService, expectedQuery, expectedQueryID int) (bool, error) {
	cfg := TestConfig()
	cfg.DNSConfig.UDPAnswerLimit = answerLimit
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	for i := 0; i < generateNumNodes; i++ {
		nodeAddress := fmt.Sprintf("127.0.0.%d", i+1)
		if rand.Float64() < pctNodesWithIPv6 {
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

		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		c := &dns.Client{Net: "udp"}
		in, _, err := c.Exchange(m, addr.String())
		if err != nil {
			return false, fmt.Errorf("err: %v", err)
		}

		switch idx {
		case 0:
			if (expectedService > 0 && len(in.Answer) != expectedService) ||
				(expectedService < -1 && len(in.Answer) < lib.AbsInt(expectedService)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s", len(in.Answer), answerLimit, qType, question)
			}
		case 1:
			if (expectedQuery > 0 && len(in.Answer) != expectedQuery) ||
				(expectedQuery < -1 && len(in.Answer) < lib.AbsInt(expectedQuery)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s", len(in.Answer), answerLimit, qType, question)
			}
		case 2:
			if (expectedQueryID > 0 && len(in.Answer) != expectedQueryID) ||
				(expectedQueryID < -1 && len(in.Answer) < lib.AbsInt(expectedQueryID)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s", len(in.Answer), answerLimit, qType, question)
			}
		default:
			panic("abort")
		}
	}

	return true, nil
}

func TestDNS_ServiceLookup_AnswerLimits(t *testing.T) {
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
		t.Run("A lookup", func(t *testing.T) {
			t.Parallel()
			ok, err := testDNS_ServiceLookup_responseLimits(t, test.udpAnswerLimit, dns.TypeA, test.expectedAService, test.expectedAQuery, test.expectedAQueryID)
			if !ok {
				t.Errorf("Expected service A lookup %s to pass: %v", test.name, err)
			}
		})

		t.Run("AAAA lookup", func(t *testing.T) {
			t.Parallel()
			ok, err := testDNS_ServiceLookup_responseLimits(t, test.udpAnswerLimit, dns.TypeAAAA, test.expectedAAAAService, test.expectedAAAAQuery, test.expectedAAAAQueryID)
			if !ok {
				t.Errorf("Expected service AAAA lookup %s to pass: %v", test.name, err)
			}
		})

		t.Run("ANY lookup", func(t *testing.T) {
			t.Parallel()
			ok, err := testDNS_ServiceLookup_responseLimits(t, test.udpAnswerLimit, dns.TypeANY, test.expectedANYService, test.expectedANYQuery, test.expectedANYQueryID)
			if !ok {
				t.Errorf("Expected service ANY lookup %s to pass: %v", test.name, err)
			}
		})
	}
}

func TestDNS_ServiceLookup_CNAME(t *testing.T) {
	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
		},
	})
	defer recursor.Shutdown()

	cfg := TestConfig()
	cfg.DNSRecursor = recursor.Addr
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
		},
	})
	defer recursor.Shutdown()

	cfg := TestConfig()
	cfg.DNSRecursor = recursor.Addr
	cfg.DNSConfig.NodeTTL = 10 * time.Second
	cfg.DNSConfig.AllowStale = Bool(true)
	cfg.DNSConfig.MaxStale = time.Second
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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

	in, _, err = c.Exchange(m, addr.String())
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

	in, _, err = c.Exchange(m, addr.String())
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
	t.Parallel()
	cfg := TestConfig()
	cfg.DNSConfig.ServiceTTL = map[string]time.Duration{
		"db": 10 * time.Second,
		"*":  5 * time.Second,
	}
	cfg.DNSConfig.AllowStale = Bool(true)
	cfg.DNSConfig.MaxStale = time.Second
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	// Register node with 2 services
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
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
			Service: "api",
			Port:    2222,
		},
	}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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
	if srvRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	aRec, ok := in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}

	m = new(dns.Msg)
	m.SetQuestion("api.service.consul.", dns.TypeSRV)
	in, _, err = c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	srvRec, ok = in.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if srvRec.Hdr.Ttl != 5 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	aRec, ok = in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Ttl != 5 {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
}

func TestDNS_PreparedQuery_TTL(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.DNSConfig.ServiceTTL = map[string]time.Duration{
		"db": 10 * time.Second,
		"*":  5 * time.Second,
	}
	cfg.DNSConfig.AllowStale = Bool(true)
	cfg.DNSConfig.MaxStale = time.Second
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	// Register a node and a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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
				Service: "api",
				Port:    2222,
			},
		}
		if err := a.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register prepared queries with and without a TTL set for "db", as
	// well as one for "api".
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "db-ttl",
				Service: structs.ServiceQuery{
					Service: "db",
				},
				DNS: structs.QueryDNSOptions{
					TTL: "18s",
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
				Name: "db-nottl",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}

		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "api-nottl",
				Service: structs.ServiceQuery{
					Service: "api",
				},
			},
		}

		if err := a.RPC("PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure the TTL is set when requested, and overrides the agent-
	// specific config since the query takes precedence.
	m := new(dns.Msg)
	m.SetQuestion("db-ttl.query.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
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
	if srvRec.Hdr.Ttl != 18 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	aRec, ok := in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Ttl != 18 {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}

	// And the TTL should take the service-specific value from the agent's
	// config otherwise.
	m = new(dns.Msg)
	m.SetQuestion("db-nottl.query.consul.", dns.TypeSRV)
	in, _, err = c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	srvRec, ok = in.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if srvRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	aRec, ok = in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}

	// If there's no query TTL and no service-specific value then the wild
	// card value should be used.
	m = new(dns.Msg)
	m.SetQuestion("api-nottl.query.consul.", dns.TypeSRV)
	in, _, err = c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	srvRec, ok = in.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if srvRec.Hdr.Ttl != 5 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	aRec, ok = in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Ttl != 5 {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
}

func TestDNS_PreparedQuery_Failover(t *testing.T) {
	t.Parallel()
	cfg1 := TestConfig()
	cfg1.Datacenter = "dc1"
	cfg1.TranslateWanAddrs = true
	cfg1.ACLDatacenter = ""
	a1 := NewTestAgent(t.Name(), cfg1)
	defer a1.Shutdown()

	cfg2 := TestConfig()
	cfg2.Datacenter = "dc2"
	cfg2.TranslateWanAddrs = true
	cfg2.ACLDatacenter = ""
	a2 := NewTestAgent(t.Name(), cfg2)
	defer a2.Shutdown()

	// Join WAN cluster.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.Ports.SerfWan)
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
					Failover: structs.QueryDatacenterOptions{
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
	cl_addr, _ := a1.Config.ClientListener("", a1.Config.Ports.DNS)
	in, _, err := c.Exchange(m, cl_addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	questions := []string{
		"_db._master.service.dc1.consul.",
		"_db._master.service.consul.",
		"_db._master.dc1.consul.",
		"_db._master.consul.",
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
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
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	cfg := TestConfig()
	cfg.ACLMasterToken = "root"
	cfg.ACLDatacenter = "dc1"
	cfg.ACLDownPolicy = "deny"
	cfg.ACLDefaultPolicy = "deny"
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

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
	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	m := new(dns.Msg)
	m.SetQuestion("foo.service.consul.", dns.TypeA)

	// Query with the root token. Should get results.
	a.Config.ACLToken = "root"
	in, _, err := c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	// Query with a non-root token without access. Should get nothing.
	a.Config.ACLToken = "anonymous"
	in, _, err = c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(in.Answer) != 0 {
		t.Fatalf("Bad: %#v", in)
	}
}

func TestDNS_AddressLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Look up the addresses
	cases := map[string]string{
		"7f000001.addr.dc1.consul.": "127.0.0.1",
	}
	for question, answer := range cases {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
		if aRec.A.To4().String() != answer {
			t.Fatalf("Bad: %#v", aRec)
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}
}

func TestDNS_AddressLookupIPV6(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Look up the addresses
	cases := map[string]string{
		"2607002040050808000000000000200e.addr.consul.": "2607:20:4005:808::200e",
		"2607112040051808ffffffffffff200e.addr.consul.": "2607:1120:4005:1808:ffff:ffff:ffff:200e",
	}
	for question, answer := range cases {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
		if aaaaRec.AAAA.To16().String() != answer {
			t.Fatalf("Bad: %#v", aaaaRec)
		}
		if aaaaRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}
}

func TestDNS_NonExistingLookup(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)

	// lookup a non-existing node, we should receive a SOA
	m := new(dns.Msg)
	m.SetQuestion("nonexisting.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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

		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		c := new(dns.Client)
		in, _, err := c.Exchange(m, addr.String())
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

	// Check for ipv4 records on ipv6-only service directly and via the
	// prepared query.
	questions = []string{
		"webv6.service.consul.",
		"webv6.query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeA)

		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		c := new(dns.Client)
		in, _, err := c.Exchange(m, addr.String())
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

func TestDNS_PreparedQuery_AllowStale(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.DNSConfig.AllowStale = Bool(true)
	cfg.DNSConfig.MaxStale = time.Second
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := MockPreparedQuery{}
	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	m.executeFn = func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
		// Return a response that's perpetually too stale.
		reply.LastContact = 2 * time.Second
		return nil
	}

	// Make sure that the lookup terminates and results in an SOA since
	// the query doesn't exist.
	{
		m := new(dns.Msg)
		m.SetQuestion("nope.query.consul.", dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Try invalid forms of queries that should hit the special invalid case
	// of our query parser.
	questions := []string{
		"consul.",
		"node.consul.",
		"service.consul.",
		"query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		in, _, err := c.Exchange(m, addr.String())
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	m := MockPreparedQuery{}
	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	m.executeFn = func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
		// Check that the agent inserted its self-name and datacenter to
		// the RPC request body.
		if args.Agent.Datacenter != a.Config.Datacenter ||
			args.Agent.Node != a.Config.NodeName {
			t.Fatalf("bad: %#v", args.Agent)
		}
		return nil
	}

	{
		m := new(dns.Msg)
		m.SetQuestion("foo.query.consul.", dns.TypeSRV)

		c := new(dns.Client)
		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		if _, _, err := c.Exchange(m, addr.String()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
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

	config := &DefaultConfig().DNSConfig
	if trimmed := trimUDPResponse(config, req, resp); trimmed {
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
	config := &DefaultConfig().DNSConfig

	req, resp, expected := &dns.Msg{}, &dns.Msg{}, &dns.Msg{}
	for i := 0; i < config.UDPAnswerLimit+1; i++ {
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
		if i < config.UDPAnswerLimit {
			expected.Answer = append(expected.Answer, srv)
			expected.Extra = append(expected.Extra, a)
		}
	}

	if trimmed := trimUDPResponse(config, req, resp); !trimmed {
		t.Fatalf("Bad %#v", *resp)
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

func TestDNS_trimUDPResponse_TrimSize(t *testing.T) {
	t.Parallel()
	config := &DefaultConfig().DNSConfig

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
	if trimmed := trimUDPResponse(config, req, resp); !trimmed {
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
	config := &DefaultConfig().DNSConfig

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
	if trimmed := trimUDPResponse(config, req, resp); !trimmed {
		t.Errorf("expected response to be trimmed: %#v", resp)
	}
	if trimmed := trimUDPResponse(config, reqEDNS, respEDNS); !trimmed {
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
	config := &DefaultConfig().DNSConfig

	req, m := dns.Msg{}, dns.Msg{}
	trimUDPResponse(config, &req, &m)
	if m.Compress {
		t.Fatalf("compression should be off")
	}

	// The trim function temporarily turns off compression, so we need to
	// make sure the setting gets restored properly.
	m.Compress = true
	trimUDPResponse(config, &req, &m)
	if !m.Compress {
		t.Fatalf("compression should be on")
	}
}

func TestDNS_Compression_Query(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"master"},
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

		addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
		conn, err := dns.Dial("udp", addr.String())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Do a manual exchange with compression on (the default).
		a.dns.config.DisableCompression = false
		if err := conn.WriteMsg(m); err != nil {
			t.Fatalf("err: %v", err)
		}
		p := make([]byte, dns.MaxMsgSize)
		compressed, err := conn.Read(p)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Disable compression and try again.
		a.dns.config.DisableCompression = true
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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

	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	conn, err := dns.Dial("udp", addr.String())
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
	a.dns.config.DisableCompression = true
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
	t.Parallel()
	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{dnsA("apple.com", "1.2.3.4")},
	})
	defer recursor.Shutdown()

	cfg := TestConfig()
	cfg.DNSRecursor = recursor.Addr
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	addr, _ := a.Config.ClientListener("", a.Config.Ports.DNS)
	conn, err := dns.Dial("udp", addr.String())
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
	a.dns.config.DisableCompression = true
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
