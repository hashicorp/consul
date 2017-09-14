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

func TestLookupWildcard(t *testing.T) {
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

	for _, lookup := range []string{"a.w.example.org.", "a.a.w.example.org."} {
		resp, err := p.Lookup(state, lookup, dns.TypeTXT)
		if err != nil || resp == nil {
			t.Fatalf("Expected to receive reply, but didn't for %s", lookup)
		}

		// ;; ANSWER SECTION:
		// a.w.example.org.          1800    IN      TXT     "Wildcard"
		if resp.Rcode != dns.RcodeSuccess {
			t.Errorf("Expected NOERROR RCODE, got %s for %s", dns.RcodeToString[resp.Rcode], lookup)
			continue
		}
		if len(resp.Answer) == 0 {
			t.Errorf("Expected to at least one RR in the answer section, got none for %s TXT", lookup)
			t.Logf("%s", resp)
			continue
		}
		if resp.Answer[0].Header().Name != lookup {
			t.Errorf("Expected name to be %s, got: %s for TXT", lookup, resp.Answer[0].Header().Name)
			continue
		}
		if resp.Answer[0].Header().Rrtype != dns.TypeTXT {
			t.Errorf("Expected RR to be TXT, got: %d, for %s TXT", resp.Answer[0].Header().Rrtype, lookup)
			continue
		}
		if resp.Answer[0].(*dns.TXT).Txt[0] != "Wildcard" {
			t.Errorf("Expected Wildcard, got: %s, for %s TXT", resp.Answer[0].(*dns.TXT).Txt[0], lookup)
			continue
		}
	}

	for _, lookup := range []string{"w.example.org.", "a.w.example.org.", "a.a.w.example.org."} {
		resp, err := p.Lookup(state, lookup, dns.TypeSRV)
		if err != nil || resp == nil {
			t.Fatal("Expected to receive reply, but didn't", lookup)
		}

		// ;; AUTHORITY SECTION:
		// example.org.              1800    IN      SOA     linode.atoom.net. miek.miek.nl. 1454960557 14400 3600 604800 14400
		if resp.Rcode != dns.RcodeSuccess {
			t.Errorf("Expected NOERROR RCODE, got %s for %s", dns.RcodeToString[resp.Rcode], lookup)
			continue
		}
		if len(resp.Answer) != 0 {
			t.Errorf("Expected zero RRs in the answer section, got some, for %s SRV", lookup)
			continue
		}
		if len(resp.Ns) == 0 {
			t.Errorf("Expected to at least one RR in the authority section, got none, for %s SRV", lookup)
			continue
		}
		if resp.Ns[0].Header().Rrtype != dns.TypeSOA {
			t.Errorf("Expected RR to be SOA, got: %d, for %s SRV", resp.Ns[0].Header().Rrtype, lookup)
			continue
		}
	}

}
