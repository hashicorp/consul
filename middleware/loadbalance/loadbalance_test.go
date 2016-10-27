package loadbalance

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestLoadBalance(t *testing.T) {
	rm := RoundRobin{Next: handler()}

	// the first X records must be cnames after this test
	tests := []struct {
		answer        []dns.RR
		extra         []dns.RR
		cnameAnswer   int
		cnameExtra    int
		addressAnswer int
		addressExtra  int
		mxAnswer      int
		mxExtra       int
	}{
		{
			answer: []dns.RR{
				newCNAME("cname1.region2.skydns.test.	300	IN	CNAME		cname2.region2.skydns.test."),
				newCNAME("cname2.region2.skydns.test.	300	IN	CNAME		cname3.region2.skydns.test."),
				newCNAME("cname5.region2.skydns.test.	300	IN	CNAME		cname6.region2.skydns.test."),
				newCNAME("cname6.region2.skydns.test.	300	IN	CNAME		endpoint.region2.skydns.test."),
				newA("endpoint.region2.skydns.test.		300	IN	A			10.240.0.1"),
				newMX("mx.region2.skydns.test.			300	IN	MX		1	mx1.region2.skydns.test."),
				newMX("mx.region2.skydns.test.			300	IN	MX		2	mx2.region2.skydns.test."),
				newMX("mx.region2.skydns.test.			300	IN	MX		3	mx3.region2.skydns.test."),
			},
			cnameAnswer:   4,
			addressAnswer: 1,
			mxAnswer:      3,
		},
		{
			answer: []dns.RR{
				newA("endpoint.region2.skydns.test.		300	IN	A			10.240.0.1"),
				newMX("mx.region2.skydns.test.			300	IN	MX		1	mx1.region2.skydns.test."),
				newCNAME("cname.region2.skydns.test.	300	IN	CNAME		endpoint.region2.skydns.test."),
			},
			cnameAnswer:   1,
			addressAnswer: 1,
			mxAnswer:      1,
		},
		{
			answer: []dns.RR{
				newMX("mx.region2.skydns.test.			300	IN	MX		1	mx1.region2.skydns.test."),
				newA("endpoint.region2.skydns.test.		300	IN	A			10.240.0.1"),
				newA("endpoint.region2.skydns.test.		300	IN	A			10.240.0.2"),
				newMX("mx.region2.skydns.test.			300	IN	MX		1	mx2.region2.skydns.test."),
				newCNAME("cname2.region2.skydns.test.	300	IN	CNAME		cname3.region2.skydns.test."),
				newA("endpoint.region2.skydns.test.		300	IN	A			10.240.0.3"),
				newMX("mx.region2.skydns.test.			300	IN	MX		1	mx3.region2.skydns.test."),
			},
			extra: []dns.RR{
				newA("endpoint.region2.skydns.test.		300	IN	A			10.240.0.1"),
				newAAAA("endpoint.region2.skydns.test.	300	IN	AAAA		::1"),
				newMX("mx.region2.skydns.test.			300	IN	MX		1	mx1.region2.skydns.test."),
				newCNAME("cname2.region2.skydns.test.	300	IN	CNAME		cname3.region2.skydns.test."),
				newMX("mx.region2.skydns.test.			300	IN	MX		1	mx2.region2.skydns.test."),
				newA("endpoint.region2.skydns.test.		300	IN	A			10.240.0.3"),
				newAAAA("endpoint.region2.skydns.test.	300	IN	AAAA		::2"),
				newMX("mx.region2.skydns.test.			300	IN	MX		1	mx3.region2.skydns.test."),
			},
			cnameAnswer:   1,
			cnameExtra:    1,
			addressAnswer: 3,
			addressExtra:  4,
			mxAnswer:      3,
			mxExtra:       3,
		},
	}

	rec := dnsrecorder.New(&test.ResponseWriter{})

	for i, test := range tests {
		req := new(dns.Msg)
		req.SetQuestion("region2.skydns.test.", dns.TypeSRV)
		req.Answer = test.answer
		req.Extra = test.extra

		_, err := rm.ServeDNS(context.TODO(), rec, req)
		if err != nil {
			t.Errorf("Test %d: Expected no error, but got %s", i, err)
			continue

		}

		cname, address, mx, sorted := countRecords(rec.Msg.Answer)
		if !sorted {
			t.Errorf("Test %d: Expected CNAMEs, then AAAAs, then MX in Answer, but got mixed", i)
		}
		if cname != test.cnameAnswer {
			t.Errorf("Test %d: Expected %d CNAMEs in Answer, but got %d", i, test.cnameAnswer, cname)
		}
		if address != test.addressAnswer {
			t.Errorf("Test %d: Expected %d A/AAAAs in Answer, but got %d", i, test.addressAnswer, address)
		}
		if mx != test.mxAnswer {
			t.Errorf("Test %d: Expected %d MXs in Answer, but got %d", i, test.mxAnswer, mx)
		}

		cname, address, mx, sorted = countRecords(rec.Msg.Extra)
		if !sorted {
			t.Errorf("Test %d: Expected CNAMEs, then AAAAs, then MX in Extra, but got mixed", i)
		}
		if cname != test.cnameExtra {
			t.Errorf("Test %d: Expected %d CNAMEs in Extra, but got %d", i, test.cnameAnswer, cname)
		}
		if address != test.addressExtra {
			t.Errorf("Test %d: Expected %d A/AAAAs in Extra, but got %d", i, test.addressAnswer, address)
		}
		if mx != test.mxExtra {
			t.Errorf("Test %d: Expected %d MXs in Extra, but got %d", i, test.mxAnswer, mx)
		}
	}
}

func countRecords(result []dns.RR) (cname int, address int, mx int, sorted bool) {
	const (
		Start = iota
		CNAMERecords
		ARecords
		MXRecords
		Any
	)

	// The order of the records is used to determine if the round-robin actually did anything.
	sorted = true
	cname = 0
	address = 0
	mx = 0
	state := Start
	for _, r := range result {
		switch r.Header().Rrtype {
		case dns.TypeCNAME:
			sorted = sorted && state <= CNAMERecords
			state = CNAMERecords
			cname++
		case dns.TypeA, dns.TypeAAAA:
			sorted = sorted && state <= ARecords
			state = ARecords
			address++
		case dns.TypeMX:
			sorted = sorted && state <= MXRecords
			state = MXRecords
			mx++
		default:
			state = Any
		}
	}
	return
}

func handler() middleware.Handler {
	return middleware.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		w.WriteMsg(r)
		return dns.RcodeSuccess, nil
	})
}

func newA(rr string) *dns.A         { r, _ := dns.NewRR(rr); return r.(*dns.A) }
func newAAAA(rr string) *dns.AAAA   { r, _ := dns.NewRR(rr); return r.(*dns.AAAA) }
func newCNAME(rr string) *dns.CNAME { r, _ := dns.NewRR(rr); return r.(*dns.CNAME) }
func newMX(rr string) *dns.MX       { r, _ := dns.NewRR(rr); return r.(*dns.MX) }
