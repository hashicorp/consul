package dnssec

import (
	"testing"
	"time"

	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

func TestZoneSigningBlackLies(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"miek.nl."})
	defer rm1()
	defer rm2()

	m := testNxdomainMsg()
	state := request.Request{Req: m}
	m = d.Sign(state, "miek.nl.", time.Now().UTC())
	if !section(m.Ns, 2) {
		t.Errorf("authority section should have 2 sig")
	}
	var nsec *dns.NSEC
	for _, r := range m.Ns {
		if r.Header().Rrtype == dns.TypeNSEC {
			nsec = r.(*dns.NSEC)
		}
	}
	if m.Rcode != dns.RcodeSuccess {
		t.Errorf("expected rcode %d, got %d", dns.RcodeSuccess, m.Rcode)
	}
	if nsec == nil {
		t.Fatalf("expected NSEC, got none")
	}
	if nsec.Hdr.Name != "ww.miek.nl." {
		t.Errorf("expected %s, got %s", "ww.miek.nl.", nsec.Hdr.Name)
	}
	if nsec.NextDomain != "\\000.ww.miek.nl." {
		t.Errorf("expected %s, got %s", "\\000.ww.miek.nl.", nsec.NextDomain)
	}
}

func testNxdomainMsg() *dns.Msg {
	return &dns.Msg{MsgHdr: dns.MsgHdr{Rcode: dns.RcodeNameError},
		Question: []dns.Question{dns.Question{Name: "ww.miek.nl.", Qclass: dns.ClassINET, Qtype: dns.TypeTXT}},
		Ns: []dns.RR{test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1461471181 14400 3600 604800 14400")},
	}
}
