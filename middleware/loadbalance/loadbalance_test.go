package loadbalance

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	coretest "github.com/miekg/coredns/middleware/testing"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestLoadBalance(t *testing.T) {
	rm := RoundRobin{Next: handler()}

	// the first X records must be cnames after this test
	tests := []struct {
		answer      []dns.RR
		extra       []dns.RR
		cnameAnswer int
		cnameExtra  int
	}{
		{
			answer: []dns.RR{
				newCNAME("cname1.region2.skydns.test.	300	IN	CNAME	cname2.region2.skydns.test."),
				newCNAME("cname2.region2.skydns.test.	300	IN	CNAME	cname3.region2.skydns.test."),
				newCNAME("cname5.region2.skydns.test.	300	IN	CNAME	cname6.region2.skydns.test."),
				newCNAME("cname6.region2.skydns.test.	300	IN	CNAME	endpoint.region2.skydns.test."),
				newA("endpoint.region2.skydns.test.	300	IN	A	10.240.0.1"),
			},
			cnameAnswer: 4,
		},
		{
			answer: []dns.RR{
				newA("endpoint.region2.skydns.test.	300	IN	A	10.240.0.1"),
				newCNAME("cname.region2.skydns.test.	300	IN	CNAME	endpoint.region2.skydns.test."),
			},
			cnameAnswer: 1,
		},
		{
			answer: []dns.RR{
				newA("endpoint.region2.skydns.test.	300	IN	A	10.240.0.1"),
				newA("endpoint.region2.skydns.test.	300	IN	A	10.240.0.2"),
				newCNAME("cname2.region2.skydns.test.	300	IN	CNAME	cname3.region2.skydns.test."),
				newA("endpoint.region2.skydns.test.	300	IN	A	10.240.0.3"),
			},
			extra: []dns.RR{
				newA("endpoint.region2.skydns.test.	300	IN	A	10.240.0.1"),
				newAAAA("endpoint.region2.skydns.test.	300	IN	AAAA	::1"),
				newCNAME("cname2.region2.skydns.test.	300	IN	CNAME	cname3.region2.skydns.test."),
				newA("endpoint.region2.skydns.test.	300	IN	A	10.240.0.3"),
				newAAAA("endpoint.region2.skydns.test.	300	IN	AAAA	::2"),
			},
			cnameAnswer: 1,
			cnameExtra:  1,
		},
	}

	rec := middleware.NewResponseRecorder(&coretest.ResponseWriter{})

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
		cname := 0
		for _, r := range rec.Msg().Answer {
			if r.Header().Rrtype != dns.TypeCNAME {
				break
			}
			cname++
		}
		if cname != test.cnameAnswer {
			t.Errorf("Test %d: Expected %d cnames in Answer, but got %d", i, test.cnameAnswer, cname)
		}
		cname = 0
		for _, r := range rec.Msg().Extra {
			if r.Header().Rrtype != dns.TypeCNAME {
				break
			}
			cname++
		}
		if cname != test.cnameExtra {
			t.Errorf("Test %d: Expected %d cname in Extra, but got %d", i, test.cnameExtra, cname)
		}
	}
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
