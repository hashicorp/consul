package test

import (
	"testing"

	"github.com/miekg/dns"
)

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

	m := new(dns.Msg)
	m.SetQuestion("17.0.0.10.in-addr.arpa.", dns.TypePTR)
	resp, err := dns.Exchange(m, udp)
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
