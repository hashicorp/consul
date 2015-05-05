package agent

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/miekg/dns"
)

func makeDNSServer(t *testing.T, config *DNSConfig, recursor *dns.Server) (string, *DNSServer) {
	if config == nil {
		config = &DNSConfig{}
	}
	recursors := []string{}
	if recursor != nil {
		recursors = append(recursors, recursor.Addr)
	}
	conf := nextConfig()
	addr, _ := conf.ClientListener(conf.Addresses.DNS, conf.Ports.DNS)
	dir, agent := makeAgent(t, conf)
	server, err := NewDNSServer(agent, config, agent.logOutput,
		conf.Domain, addr.String(), recursors)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}

// makeRecursor creates a generic DNS server which always returns
// the provided reply. This is useful for mocking a DNS recursor with
// an expected result.
func makeRecursor(t *testing.T, answer []dns.RR) *dns.Server {
	dnsConf := nextConfig()
	dnsAddr := fmt.Sprintf("%s:%d", dnsConf.Addresses.DNS, dnsConf.Ports.DNS)
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(resp dns.ResponseWriter, msg *dns.Msg) {
		ans := &dns.Msg{Answer: answer[:]}
		ans.SetReply(msg)
		if err := resp.WriteMsg(ans); err != nil {
			t.Fatalf("err: %s", err)
		}
	})
	server := &dns.Server{
		Addr:    dnsAddr,
		Net:     "udp",
		Handler: mux,
	}
	go server.ListenAndServe()
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
	addr, err := recursorAddr("8.8.8.8")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if addr != "8.8.8.8:53" {
		t.Fatalf("bad: %v", addr)
	}
}

func TestDNS_NodeLookup(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
}

func TestDNS_CaseInsensitiveNodeLookup(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "Foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("fOO.node.dc1.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("empty lookup: %#v", in)
	}
}

func TestDNS_NodeLookup_PeriodName(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node with period in name
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo.bar",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.bar.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "::4242:4242",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
	recursor := makeRecursor(t, []dns.RR{
		dnsCNAME("www.google.com", "google.com"),
		dnsA("google.com", "1.2.3.4"),
	})
	defer recursor.Shutdown()

	dir, srv := makeDNSServer(t, nil, recursor)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "www.google.com",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_ReverseLookup(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo2",
		Address:    "127.0.0.2",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("2.0.0.127.in-addr.arpa.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()
	srv.domain = dns.Fqdn("custom")

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo2",
		Address:    "127.0.0.2",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("2.0.0.127.in-addr.arpa.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "::4242:4242",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("2.4.2.4.2.4.2.4.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.ip6.arpa.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_ServiceLookup_ServiceAddress(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
	if aRec.A.String() != "127.0.0.2" {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Ttl != 0 {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
}

func TestDNS_CaseInsensitiveServiceLookup(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("mASTER.dB.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
	in, _, err := c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("empty lookup: %#v", in)
	}
}

func TestDNS_ServiceLookup_TagPeriod(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("v1.master.db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_ServiceLookup_Dedup_SRV(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_Recurse(t *testing.T) {
	recursor := makeRecursor(t, []dns.RR{dnsA("apple.com", "1.2.3.4")})
	defer recursor.Shutdown()

	dir, srv := makeDNSServer(t, nil, recursor)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_ServiceLookup_FilterCritical(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register nodes
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
			Status:  structs.HealthCritical,
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
			Status:  structs.HealthCritical,
		},
	}
	if err := srv.agent.RPC("Catalog.Register", args2, &out); err != nil {
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
			Status:    structs.HealthCritical,
		},
	}
	if err := srv.agent.RPC("Catalog.Register", args3, &out); err != nil {
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
	if err := srv.agent.RPC("Catalog.Register", args4, &out); err != nil {
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
			Status:    structs.HealthWarning,
		},
	}
	if err := srv.agent.RPC("Catalog.Register", args5, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_ServiceLookup_OnlyPassing(t *testing.T) {
	dir, srv := makeDNSServer(t, &DNSConfig{OnlyPassing: true}, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register nodes
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
			Status:    structs.HealthPassing,
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
			Status:    structs.HealthWarning,
		},
	}

	if err := srv.agent.RPC("Catalog.Register", args2, &out); err != nil {
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
			Status:    structs.HealthCritical,
		},
	}

	if err := srv.agent.RPC("Catalog.Register", args3, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args4 := &structs.RegisterRequest{
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
			Status:    structs.HealthUnknown,
		},
	}

	if err := srv.agent.RPC("Catalog.Register", args4, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_ServiceLookup_Randomize(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register nodes
	for i := 0; i < 3*maxServiceResponses; i++ {
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
		if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Ensure the response is randomized each time.
	uniques := map[string]struct{}{}
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
	for i := 0; i < 10; i++ {
		m := new(dns.Msg)
		m.SetQuestion("web.service.consul.", dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, addr.String())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Response length should be truncated
		// We should get an A record for each response
		if len(in.Answer) != maxServiceResponses {
			t.Fatalf("Bad: %#v", len(in.Answer))
		}

		// Collect all the names
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

		// Tally the results
		uniques[nameS] = struct{}{}
	}

	// Give some wiggle room. Since the responses are randomized and there
	// is a finite number of combinations, requiring 0 duplicates every
	// test run eventually gives us failures.
	if len(uniques) < 2 {
		t.Fatalf("unique response ratio too low: %d/10\n%v", len(uniques), uniques)
	}
}

func TestDNS_ServiceLookup_Truncate(t *testing.T) {
	config := &DNSConfig{
		EnableTruncate: true,
	}
	dir, srv := makeDNSServer(t, config, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register nodes
	for i := 0; i < 3*maxServiceResponses; i++ {
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
		if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Ensure the response is randomized each time.
	m := new(dns.Msg)
	m.SetQuestion("web.service.consul.", dns.TypeANY)

	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
	c := new(dns.Client)
	in, _, err := c.Exchange(m, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check for the truncate bit
	if !in.Truncated {
		t.Fatalf("should have truncate bit")
	}
}

func TestDNS_ServiceLookup_CNAME(t *testing.T) {
	recursor := makeRecursor(t, []dns.RR{
		dnsCNAME("www.google.com", "google.com"),
		dnsA("google.com", "1.2.3.4"),
	})
	defer recursor.Shutdown()

	dir, srv := makeDNSServer(t, nil, recursor)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("search.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_NodeLookup_TTL(t *testing.T) {
	recursor := makeRecursor(t, []dns.RR{
		dnsCNAME("www.google.com", "google.com"),
		dnsA("google.com", "1.2.3.4"),
	})
	defer recursor.Shutdown()

	config := &DNSConfig{
		NodeTTL:    10 * time.Second,
		AllowStale: true,
		MaxStale:   time.Second,
	}

	dir, srv := makeDNSServer(t, config, recursor)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
	config := &DNSConfig{
		ServiceTTL: map[string]time.Duration{
			"db": 10 * time.Second,
			"*":  5 * time.Second,
		},
		AllowStale: true,
		MaxStale:   time.Second,
	}

	dir, srv := makeDNSServer(t, config, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_ServiceLookup_SRV_RFC(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("_db._master.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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

func TestDNS_ServiceLookup_SRV_RFC_TCP_Default(t *testing.T) {
	dir, srv := makeDNSServer(t, nil, nil)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("_db._tcp.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	addr, _ := srv.agent.config.ClientListener("", srv.agent.config.Ports.DNS)
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
