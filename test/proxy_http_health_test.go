package test

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

func TestProxyWithHTTPCheckOK(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	healthCheckServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "OK\n")
		}))
	defer healthCheckServer.Close()

	healthCheckURL, err := url.Parse(healthCheckServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	// TODO: use URL.Port() (Go 1.8+) once we've deprecated Go 1.7 support
	var healthCheckPort string
	if _, healthCheckPort, err = net.SplitHostPort(healthCheckURL.Host); err != nil {
		healthCheckPort = "80"
	}

	name, rm, err := test.TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("failed to create zone: %s", err)
	}
	defer rm()

	// We have to bind to 127.0.0.1 because the server started by
	// httptest.NewServer does, and the IP addresses of the backend
	// DNS and HTTP servers must match.
	authoritativeCorefile := `example.org:0 {
	   bind 127.0.0.1
       file ` + name + `
}
`

	authoritativeInstance, authoritativeAddr, _, err := CoreDNSServerAndPorts(authoritativeCorefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS authoritative instance: %s", err)
	}
	defer authoritativeInstance.Stop()

	proxyCorefile := `example.org:0 {
    proxy . ` + authoritativeAddr + ` {
		health_check /health:` + healthCheckPort + ` 1s

	}
}
`

	proxyInstance, proxyAddr, _, err := CoreDNSServerAndPorts(proxyCorefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS proxy instance: %s", err)
	}
	defer proxyInstance.Stop()

	p := proxy.NewLookup([]string{proxyAddr})
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
