package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestLookupProxy(t *testing.T) {
	t.Parallel()
	name, rm, err := test.TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("failed to create zone: %s", err)
	}
	defer rm()

	corefile := `example.org:0 {
       file ` + name + `
}
`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	log.SetOutput(ioutil.Discard)

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	resp, err := p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	// expect answer section with A record in it
	if len(resp.Answer) == 0 {
		t.Fatalf("Expected to at least one RR in the answer section, got none: %s", resp)
	}
	if resp.Answer[0].Header().Rrtype != dns.TypeA {
		t.Errorf("Expected RR to A, got: %d", resp.Answer[0].Header().Rrtype)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.0.0.1" {
		t.Errorf("Expected 127.0.0.1, got: %s", resp.Answer[0].(*dns.A).A.String())
	}
}

func TestLookupDnsWithForcedTcp(t *testing.T) {
	t.Parallel()
	name, rm, err := test.TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("failed to create zone: %s", err)
	}
	defer rm()

	corefile := `example.org:0 {
       file ` + name + `
}
`

	i, _, tcp, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	log.SetOutput(ioutil.Discard)

	p := proxy.NewLookupWithOption([]string{tcp}, proxy.Options{ForceTCP: true})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	resp, err := p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	// expect answer section with A record in it
	if len(resp.Answer) == 0 {
		t.Fatalf("Expected to at least one RR in the answer section, got none: %s", resp)
	}
	if resp.Answer[0].Header().Rrtype != dns.TypeA {
		t.Errorf("Expected RR to A, got: %d", resp.Answer[0].Header().Rrtype)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.0.0.1" {
		t.Errorf("Expected 127.0.0.1, got: %s", resp.Answer[0].(*dns.A).A.String())
	}
}

func BenchmarkProxyLookup(b *testing.B) {
	t := new(testing.T)
	name, rm, err := test.TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("failed to created zone: %s", err)
	}
	defer rm()

	corefile := `example.org:0 {
       file ` + name + `
}
`

	i, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatalf("could not get udp listening port")
	}
	defer i.Stop()

	log.SetOutput(ioutil.Discard)

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Lookup(state, "example.org.", dns.TypeA)
		if err != nil {
			b.Fatal("Expected to receive reply, but didn't")
		}
	}
}
