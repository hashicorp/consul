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

func TestReverseFallthrough(t *testing.T) {
	t.Parallel()
	name, rm, err := test.TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("failed to create zone: %s", err)
	}
	defer rm()

	corefile := `arpa:0 example.org:0 {
    reverse 10.32.0.0/16 {
        hostname ip-{ip}.{zone[2]}
        #fallthrough
    }
    file ` + name + ` example.org
}
`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	log.SetOutput(ioutil.Discard)

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	resp, err := p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	// Reply should be SERVFAIL because of no fallthrough
	if resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("Expected SERVFAIL, but got: %d", resp.Rcode)
	}

	// Stop the server.
	i.Stop()

	// And redo with fallthrough enabled

	corefile = `arpa:0 example.org:0 {
    reverse 10.32.0.0/16 {
        hostname ip-{ip}.{zone[2]}
        fallthrough
    }
    file ` + name + ` example.org
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

	p = proxy.NewLookup([]string{udp})
	resp, err = p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}

	if len(resp.Answer) == 0 {
		t.Error("Expected to at least one RR in the answer section, got none")
	}
	if resp.Answer[0].Header().Rrtype != dns.TypeA {
		t.Errorf("Expected RR to A, got: %d", resp.Answer[0].Header().Rrtype)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.0.0.1" {
		t.Errorf("Expected 127.0.0.1, got: %s", resp.Answer[0].(*dns.A).A.String())
	}
}

func TestReverseCorefile(t *testing.T) {
	corefile := `10.0.0.0/24:0 {
		whoami
	}`

	i, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	udp, _ := CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
	}

	log.SetOutput(ioutil.Discard)

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	resp, err := p.Lookup(state, "17.0.0.10.in-addr.arpa.", dns.TypePTR)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}

	if len(resp.Extra) != 2 {
		t.Fatal("Expected to at least two RRs in the extra section, got none")
	}
	// Second one is SRV, first one can be A or AAAA depending on system.
	if resp.Extra[1].Header().Rrtype != dns.TypeSRV {
		t.Errorf("Expected RR to SRV, got: %d", resp.Extra[1].Header().Rrtype)
	}
	if resp.Extra[1].Header().Name != "_udp.17.0.0.10.in-addr.arpa." {
		t.Errorf("Expected _udp.17.0.0.10.in-addr.arpa. got: %s", resp.Extra[1].Header().Name)
	}
}
