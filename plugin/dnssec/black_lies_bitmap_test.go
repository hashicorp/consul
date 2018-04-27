package dnssec

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

const server = "dns//."

func TestBlackLiesBitmapNoData(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"example.org."})
	defer rm1()
	defer rm2()

	m := testTLSAMsg()
	state := request.Request{Req: m, Zone: "example.org."}
	m = d.Sign(state, time.Now().UTC(), server)

	var nsec *dns.NSEC
	for _, r := range m.Ns {
		if r.Header().Rrtype == dns.TypeNSEC {
			nsec = r.(*dns.NSEC)
		}
	}
	for _, b := range nsec.TypeBitMap {
		if uint16(b) == dns.TypeTLSA {
			t.Errorf("Type TLSA should not be present in the type bitmap: %v", nsec.TypeBitMap)
		}
	}
}
func TestBlackLiesBitmapNameError(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"example.org."})
	defer rm1()
	defer rm2()

	m := testTLSAMsg()
	m.Rcode = dns.RcodeNameError // change to name error
	state := request.Request{Req: m, Zone: "example.org."}
	m = d.Sign(state, time.Now().UTC(), server)

	var nsec *dns.NSEC
	for _, r := range m.Ns {
		if r.Header().Rrtype == dns.TypeNSEC {
			nsec = r.(*dns.NSEC)
		}
	}
	for _, b := range nsec.TypeBitMap {
		if uint16(b) == dns.TypeTLSA {
			t.Errorf("Type TLSA should not be present in the type bitmap: %v", nsec.TypeBitMap)
		}
	}
}

func testTLSAMsg() *dns.Msg {
	return &dns.Msg{MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Question: []dns.Question{{Name: "25._tcp.example.org.", Qclass: dns.ClassINET, Qtype: dns.TypeTLSA}},
		Ns: []dns.RR{test.SOA("example.org.	1800	IN	SOA	linode.example.org. miek.example.org. 1461471181 14400 3600 604800 14400")},
	}
}
