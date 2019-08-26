package test

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestEmptySecondaryZone(t *testing.T) {
	// Corefile that fails to transfer example.org.
	corefile := `example.org:0 {
		secondary {
			transfer from 127.0.0.1:1717
		}
	}
`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	m := new(dns.Msg)
	m.SetQuestion("www.example.org.", dns.TypeA)
	resp, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	if resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("Expected reply to be a SERVFAIL, got %d", resp.Rcode)
	}
}

func TestSecondaryZoneTransfer(t *testing.T) {
	name, rm, err := test.TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	corefile := `example.org:0 {
       file ` + name + ` {
	       transfer to *
       }
}
`

	i, _, tcp, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	corefile = `example.org:0 {
		secondary {
			transfer from ` + tcp + `
		}
}
`
	i1, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i1.Stop()

	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeSOA)

	var r *dns.Msg
	// This is now async; we need to wait for it to be transferred.
	for i := 0; i < 10; i++ {
		r, _ = dns.Exchange(m, udp)
		if len(r.Answer) != 0 {
			break
		}
		time.Sleep(100 * time.Microsecond)
	}
	if len(r.Answer) == 0 {
		t.Fatalf("Expected answer section")
	}
}

func TestIxfrResponse(t *testing.T) {
	// ixfr query with current soa should return single packet with that soa (no transfer needed).
	name, rm, err := test.TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	corefile := `example.org:0 {
       file ` + name + ` {
	       transfer to *
       }
}
`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeIXFR)
	m.Ns = []dns.RR{test.SOA("example.org. IN SOA sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600")} // copied from exampleOrg

	var r *dns.Msg
	// This is now async; we need to wait for it to be transferred.
	for i := 0; i < 10; i++ {
		r, _ = dns.Exchange(m, udp)
		if len(r.Answer) != 0 {
			break
		}
		time.Sleep(100 * time.Microsecond)
	}
	if len(r.Answer) != 1 {
		t.Fatalf("Expected answer section with single RR")
	}
	soa, ok := r.Answer[0].(*dns.SOA)
	if !ok {
		t.Fatalf("Expected answer section with SOA RR")
	}
	if soa.Serial != 2015082541 {
		t.Fatalf("Serial should be %d, got %d", 2015082541, soa.Serial)
	}
}
