package dnssec

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestZoneSigningBlackLies(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"miek.nl."})
	defer rm1()
	defer rm2()

	m := testNxdomainMsg()
	state := request.Request{Req: m, Zone: "miek.nl."}
	m = d.Sign(state, time.Now().UTC(), server)
	if !section(m.Ns, 2) {
		t.Errorf("Authority section should have 2 sigs")
	}
	var nsec *dns.NSEC
	for _, r := range m.Ns {
		if r.Header().Rrtype == dns.TypeNSEC {
			nsec = r.(*dns.NSEC)
		}
	}
	if m.Rcode != dns.RcodeSuccess {
		t.Errorf("Expected rcode %d, got %d", dns.RcodeSuccess, m.Rcode)
	}
	if nsec == nil {
		t.Fatalf("Expected NSEC, got none")
	}
	if nsec.Hdr.Name != "ww.miek.nl." {
		t.Errorf("Expected %s, got %s", "ww.miek.nl.", nsec.Hdr.Name)
	}
	if nsec.NextDomain != "\\000.ww.miek.nl." {
		t.Errorf("Expected %s, got %s", "\\000.ww.miek.nl.", nsec.NextDomain)
	}
}

func TestBlackLiesNoError(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"miek.nl."})
	defer rm1()
	defer rm2()

	m := testSuccessMsg()
	state := request.Request{Req: m, Zone: "miek.nl."}
	m = d.Sign(state, time.Now().UTC(), server)

	if m.Rcode != dns.RcodeSuccess {
		t.Errorf("Expected rcode %d, got %d", dns.RcodeSuccess, m.Rcode)
	}

	if len(m.Answer) != 2 {
		t.Errorf("Answer section should have 2 RRs")
	}
	sig, txt := false, false
	for _, rr := range m.Answer {
		if _, ok := rr.(*dns.RRSIG); ok {
			sig = true
		}
		if _, ok := rr.(*dns.TXT); ok {
			txt = true
		}
	}
	if !sig || !txt {
		t.Errorf("Expected RRSIG and TXT in answer section")
	}
}

func testNxdomainMsg() *dns.Msg {
	return &dns.Msg{MsgHdr: dns.MsgHdr{Rcode: dns.RcodeNameError},
		Question: []dns.Question{{Name: "ww.miek.nl.", Qclass: dns.ClassINET, Qtype: dns.TypeTXT}},
		Ns: []dns.RR{test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1461471181 14400 3600 604800 14400")},
	}
}

func testSuccessMsg() *dns.Msg {
	return &dns.Msg{MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Question: []dns.Question{{Name: "www.miek.nl.", Qclass: dns.ClassINET, Qtype: dns.TypeTXT}},
		Answer: []dns.RR{test.TXT(`www.miek.nl.	1800	IN	TXT	"response"`)},
	}
}
