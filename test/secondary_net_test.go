// +build net

package test

import (
	"testing"

	"github.com/miekg/dns"
)

func TestSecondaryZoneTransfer(t *testing.T) {
	/*
		Test will only work when there is a CoreDNS running on part 32054
		with example.com and willing to transfer
		coredns -conf Corefile -dns.port 32054
		Corefile:
			example.com {
			    file example.com {
				transfer to 127.0.0.1:32053
			    }
			}
		example.com:
			example.com.		3600	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2017042730 7200 3600 1209600 3600

			example.com.		65118	IN	NS	a.iana-servers.net.
			example.com.		65118	IN	NS	b.iana-servers.net.
			cname.example.com.       434334  IN      CNAME   a.miek.nl.
	*/

	corefile := `example.com:32053 {
		secondary {
			transfer from 127.0.0.1:32054
		}
	}
	`

	sec, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	defer sec.Stop()

	m := new(dns.Msg)
	m.SetQuestion("cname.example.com.", dns.TypeCNAME)

	r, err := dns.Exchange(m, "127.0.0.1:32053")
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}

	if len(r.Answer) == 0 {
		t.Fatalf("Expected answer section")
	}

	if r.Answer[0].(*dns.CNAME).Target != "a.miek.nl." {
		t.Fatalf("Expected target of %s, got %s", "a.miek.nl.", r.Answer[0].(*dns.CNAME).Target)
	}

	m = new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeSOA)
	r, err = dns.Exchange(m, "127.0.0.1:32053")
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}
	if len(r.Answer) == 0 {
		t.Fatalf("Expected answer section")
	}
	if r.Answer[0].(*dns.SOA).Serial != 2017042730 {
		t.Fatalf("Expected serial of %d, got %d", 2017042730, r.Answer[0].(*dns.SOA).Serial)
	}
}

func TestSecondaryZoneTransferUpstream(t *testing.T) {
	/*
		Test will only work when there is a CoreDNS running on part 32054
		with example.com and willing to transfer
		coredns -conf Corefile -dns.port 32054
		Corefile:
			example.com {
			    file example.com {
				transfer to 127.0.0.1:32053
			    }
			}
		example.com:
			example.com.		3600	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2017042730 7200 3600 1209600 3600

			example.com.		65118	IN	NS	a.iana-servers.net.
			example.com.		65118	IN	NS	b.iana-servers.net.
			cname.example.com.       434334  IN      CNAME   a.miek.nl.
	*/

	corefile := `example.com:32053 {
		secondary {
			transfer from 127.0.0.1:32054
			upstream 8.8.8.8
		}
	}
	`

	sec, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	defer sec.Stop()

	m := new(dns.Msg)
	m.SetQuestion("cname.example.com.", dns.TypeA)

	r, err := dns.Exchange(m, "127.0.0.1:32053")
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}

	if len(r.Answer) != 2 {
		t.Fatalf("Expected answer section, with 2 records, got %d", len(r.Answer))
	}

	if r.Answer[0].(*dns.CNAME).Target != "a.miek.nl." {
		t.Fatalf("Expected target of %s, got %s", "a.miek.nl.", r.Answer[0].(*dns.CNAME).Target)
	}
	if r.Answer[1].Header().Name != "a.miek.nl." {
		t.Fatalf("Expected name of %s, got %s", "a.miek.nl.", r.Answer[1].Header().Name)
	}
}
