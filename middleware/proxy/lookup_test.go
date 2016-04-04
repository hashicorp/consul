package proxy

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/miekg/coredns/middleware"
	coretest "github.com/miekg/coredns/middleware/testing"

	"github.com/miekg/dns"
)

func TestLookupProxy(t *testing.T) {
	// TODO(miek): make this fakeDNS backend and ask the question locally
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stderr)

	p := New([]string{"8.8.8.8:53"})
	resp, err := p.Lookup(fakeState(), "example.org.", dns.TypeA)
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
}

func fakeState() middleware.State {
	return middleware.State{W: &coretest.ResponseWriter{}, Req: new(dns.Msg)}
}
