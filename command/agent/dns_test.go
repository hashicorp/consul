package agent

import (
	"github.com/hashicorp/consul/consul/structs"
	"github.com/miekg/dns"
	"os"
	"testing"
	"time"
)

func makeDNSServer(t *testing.T) (string, *DNSServer) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	server, err := NewDNSServer(agent, agent.logOutput, conf.Domain,
		conf.DNSAddr, "8.8.8.8:53")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}

func TestDNS_IsAlive(t *testing.T) {
	dir, srv := makeDNSServer(t)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("_test.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, srv.agent.config.DNSAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	txt, ok := in.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if txt.Txt[0] != "ok" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_NodeLookup(t *testing.T) {
	dir, srv := makeDNSServer(t)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

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
	in, _, err := c.Exchange(m, srv.agent.config.DNSAddr)
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

	// Re-do the query, but specify the DC
	m = new(dns.Msg)
	m.SetQuestion("foo.node.dc1.consul.", dns.TypeANY)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, srv.agent.config.DNSAddr)
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
}

func TestDNS_ServiceLookup(t *testing.T) {
	dir, srv := makeDNSServer(t)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Register node
	args := &structs.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 12345,
	}
	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, srv.agent.config.DNSAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 2 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok := in.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	srvRec, ok := in.Answer[1].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[1])
	}
	if srvRec.Port != 12345 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Target != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", srvRec)
	}

	aRec, ok = in.Extra[0].(*dns.A)
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
	dir, srv := makeDNSServer(t)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Register node
	args := &structs.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 12345,
	}
	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args = &structs.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceID:   "db2",
		ServiceName: "db",
		ServiceTag:  "slave",
		ServicePort: 12345,
	}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args = &structs.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceID:   "db3",
		ServiceName: "db",
		ServiceTag:  "slave",
		ServicePort: 12346,
	}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, srv.agent.config.DNSAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 3 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok := in.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	srvRec, ok := in.Answer[1].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[1])
	}
	if srvRec.Port != 12345 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Target != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", srvRec)
	}

	srvRec, ok = in.Answer[2].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[1])
	}
	if srvRec.Port != 12346 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Target != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", srvRec)
	}

	aRec, ok = in.Extra[0].(*dns.A)
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
	dir, srv := makeDNSServer(t)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("apple.com.", dns.TypeANY)

	c := new(dns.Client)
	c.Net = "tcp"
	in, _, err := c.Exchange(m, srv.agent.config.DNSAddr)
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
