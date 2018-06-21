package test

import (
	"testing"

	"github.com/miekg/dns"
)

func TestTemplateUpstream(t *testing.T) {
	corefile := `.:0 {
 		# CNAME
		template IN ANY cname.example.net. {
			match ".*"
			answer "cname.example.net. 60 IN CNAME target.example.net."
			upstream
		}

		# Target
		template IN A target.example.net. {
			match ".*"
			answer "target.example.net. 60 IN A 1.2.3.4"
		}
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	// Test that an A query returns a CNAME and an A record
	m := new(dns.Msg)
	m.SetQuestion("cname.example.net.", dns.TypeA)
	m.SetEdns0(4096, true) // need this?

	r, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not send msg: %s", err)
	}
	if r.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Rcode should not be dns.RcodeServerFailure")
	}
	if len(r.Answer) < 2 {
		t.Fatalf("Expected 2 answers, got %v", len(r.Answer))
	}
	if x := r.Answer[0].(*dns.CNAME).Target; x != "target.example.net." {
		t.Fatalf("Failed to get address for CNAME, expected target.example.net. got %s", x)
	}
	if x := r.Answer[1].(*dns.A).A.String(); x != "1.2.3.4" {
		t.Fatalf("Failed to get address for CNAME, expected 1.2.3.4 got %s", x)
	}

	// Test that a CNAME query only returns a CNAME
	m = new(dns.Msg)
	m.SetQuestion("cname.example.net.", dns.TypeCNAME)
	m.SetEdns0(4096, true) // need this?

	r, err = dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not send msg: %s", err)
	}
	if r.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Rcode should not be dns.RcodeServerFailure")
	}
	if len(r.Answer) < 1 {
		t.Fatalf("Expected 1 answer, got %v", len(r.Answer))
	}
	if x := r.Answer[0].(*dns.CNAME).Target; x != "target.example.net." {
		t.Fatalf("Failed to get address for CNAME, expected target.example.net. got %s", x)
	}
}
