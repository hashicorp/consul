package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

func TestProxyErratic(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	corefile := `example.org:0 {
		erratic {
			drop 2
		}
	}
`

	backend, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(backend, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
	}
	defer backend.Stop()

	p := proxy.New([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	// We do one lookup that should not time out.
	// After this the backend is marked unhealthy anyway. So basically this
	// tests that it times out.
	p.Lookup(state, "example.org.", dns.TypeA)
}
