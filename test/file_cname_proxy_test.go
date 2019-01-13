package test

import (
	"testing"

	"github.com/miekg/dns"
)

func TestZoneExternalCNAMELookupWithoutProxy(t *testing.T) {
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
	m.SetQuestion("cname.example.org.", dns.TypeA)
	resp, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}
	// There should only be a CNAME in the answer section.
	if len(resp.Answer) != 1 {
		t.Fatalf("Expected 1 RR in answer section got %d", len(resp.Answer))
	}
}

func TestZoneExternalCNAMELookupWithProxy(t *testing.T) {
	t.Parallel()

	name, rm, err := TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	// Corefile with for example proxy section.
	corefile := `.:0 {
       file ` + name + ` example.org {
	       upstream
	}
	proxy . 8.8.8.8 8.8.4.4
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	m := new(dns.Msg)
	m.SetQuestion("cname.example.org.", dns.TypeA)
	resp, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}
	// There should be a CNAME *and* an IP address in the answer section.
	// For now, just check that we have 2 RRs
	if len(resp.Answer) != 2 {
		t.Fatalf("Expected 2 RRs in answer section got %d", len(resp.Answer))
	}
}
