package test

import (
	"testing"

	"github.com/miekg/dns"
)

func TestZoneSRVAdditional(t *testing.T) {
	t.Parallel()

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

	m := new(dns.Msg)
	m.SetQuestion("service.example.org.", dns.TypeSRV)
	resp, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}

	// There should be 2 A records in the additional section.
	if len(resp.Extra) != 2 {
		t.Fatalf("Expected 2 RR in additional section got %d", len(resp.Extra))
	}
}
