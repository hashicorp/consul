package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/coredns/coredns/middleware/proxy"
	"github.com/coredns/coredns/middleware/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestZoneExternalCNAMELookup(t *testing.T) {
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
	i, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
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
