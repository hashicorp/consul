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

func TestZoneExternalCNAMELookupWithoutProxy(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	name, rm, err := TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	// Corefile with for example without proxy section.
	corefile := `example.org:0 {
       file ` + name + `
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := p.Lookup(state, "cname.example.org.", dns.TypeA)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}
	// There should only be a CNAME in the answer section.
	if len(resp.Answer) != 1 {
		t.Fatalf("Expected 1 RR in answer section got %d", len(resp.Answer))
	}
}

func TestZoneExternalCNAMELookupWithProxy(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	name, rm, err := TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	// Corefile with for example without proxy section.
	corefile := `example.org:0 {
       file ` + name + ` {
	       upstream 8.8.8.8
	}
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := p.Lookup(state, "cname.example.org.", dns.TypeA)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}
	// There should be a CNAME *and* an IP address in the answer section.
	// For now, just check that we have 2 RRs
	if len(resp.Answer) != 2 {
		t.Fatalf("Expected 2 RRs in answer section got %d", len(resp.Answer))
	}
}
