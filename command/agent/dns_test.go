package agent

import (
	"github.com/miekg/dns"
	"os"
	"testing"
)

func makeDNSServer(t *testing.T) (string, *DNSServer) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	server, err := NewDNSServer(agent, agent.logOutput, conf.DNSAddr)
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
