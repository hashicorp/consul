package test

import (
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

func TestLookupCache(t *testing.T) {
	// Start auth. CoreDNS holding the auth zone.
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
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
	}
	defer i.Stop()

	// Start caching proxy CoreDNS that we want to test.
	corefile = `example.org:0 {
	proxy . ` + udp + `
	cache
}
`
	i, err = CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ = CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
	}
	defer i.Stop()

	log.SetOutput(ioutil.Discard)

	p := proxy.New([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	// expect answer section with A record in it
	if len(resp.Answer) == 0 {
		t.Error("Expected to at least one RR in the answer section, got none")
	}

	ttl := resp.Answer[0].Header().Ttl

	time.Sleep(2 * time.Second) // TODO(miek): meh.

	resp, err = p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}

	// expect answer section with A record in it
	if len(resp.Answer) == 0 {
		t.Error("Expected to at least one RR in the answer section, got none")
	}
	newTTL := resp.Answer[0].Header().Ttl
	if newTTL >= ttl {
		t.Errorf("Expected TTL to be lower than: %d, got %d", ttl, newTTL)
	}
}
