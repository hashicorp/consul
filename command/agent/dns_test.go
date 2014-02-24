package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/miekg/dns"
	"os"
	"strings"
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

func TestRecursorAddr(t *testing.T) {
	addr, err := recursorAddr("8.8.8.8")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if addr != "8.8.8.8:53" {
		t.Fatalf("bad: %v", addr)
	}
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
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tag:     "master",
			Port:    12345,
		},
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
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tag:     "master",
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
			Tag:     "slave",
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
			Tag:     "slave",
			Port:    12346,
		},
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
	if srvRec.Port != 12345 && srvRec.Port != 12346 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Target != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", srvRec)
	}

	srvRec, ok = in.Answer[2].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[1])
	}
	if srvRec.Port != 12346 && srvRec.Port != 12345 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Port == in.Answer[1].(*dns.SRV).Port {
		t.Fatalf("should be a different port")
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

func TestDNS_ServiceLookup_FilterCritical(t *testing.T) {
	dir, srv := makeDNSServer(t)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Register nodes
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tag:     "master",
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
			Tag:     "master",
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

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, srv.agent.config.DNSAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should get no answer since we are failing!
	if len(in.Answer) != 0 {
		t.Fatalf("Bad: %#v", in)
	}
}

func TestDNS_ServiceLookup_Randomize(t *testing.T) {
	dir, srv := makeDNSServer(t)
	defer os.RemoveAll(dir)
	defer srv.agent.Shutdown()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

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

	// Ensure the response is randomized each time
	uniques := map[string]struct{}{}
	for i := 0; i < 5; i++ {
		m := new(dns.Msg)
		m.SetQuestion("web.service.consul.", dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, srv.agent.config.DNSAddr)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Response length should be truncated
		// We should get an SRV + A record for each response (hence 2x)
		if len(in.Answer) != 2*maxServiceResponses {
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

		// Check if unique
		if _, ok := uniques[nameS]; ok {
			t.Fatalf("non-unique response: %v", nameS)
		}
		uniques[nameS] = struct{}{}
	}
}
