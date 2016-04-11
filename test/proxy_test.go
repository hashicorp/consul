package test

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
)

const exampleOrg = `; example.org test file
example.org.		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600
example.org.		IN	NS	b.iana-servers.net.
example.org.		IN	NS	a.iana-servers.net.
example.org.		IN	A	127.0.0.1
`

func TestLookupProxy(t *testing.T) {
	name, rm, err := test.Zone(t, ".", exampleOrg)
	if err != nil {
		t.Fatalf("failed to created zone: %s", err)
	}
	defer rm()

	corefile := `example.org:0 {
	file ` + name + `
}
`
	ex, _, udp, err := Server(t, corefile)
	if err != nil {
		t.Fatalf("Could get server: %s", err)
	}
	defer ex.Stop()

	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stderr)

	p := proxy.New([]string{udp})
	state := middleware.State{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	resp, err := p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Error("Expected to receive reply, but didn't")
	}
	// expect answer section with A record in it
	if len(resp.Answer) == 0 {
		t.Error("Expected to at least one RR in the answer section, got none")
	}
	if resp.Answer[0].Header().Rrtype != dns.TypeA {
		t.Error("Expected RR to A, got: %d", resp.Answer[0].Header().Rrtype)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.0.0.1" {
		t.Error("Expected 127.0.0.1, got: %d", resp.Answer[0].(*dns.A).A.String())
	}
}
