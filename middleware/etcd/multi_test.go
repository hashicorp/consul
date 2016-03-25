// +build etcd

package etcd

// etcd needs to be running on http://127.0.0.1:2379
// *and* needs connectivity to the internet for remotely resolving
// names.

import (
	"sort"
	"testing"

	"golang.org/x/net/context"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"

	"github.com/miekg/dns"
)

func TestMultiLookup(t *testing.T) {
	etcMulti := etc
	etcMulti.Zones = []string{"skydns.test.", "miek.nl."}
	etcMulti.Next = handler()

	for _, serv := range servicesMulti {
		set(t, etcMulti, serv.Key, 0, serv)
		defer delete(t, etcMulti, serv.Key)
	}
	for _, tc := range dnsTestCasesMulti {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(tc.Qname), tc.Qtype)

		rec := middleware.NewResponseRecorder(&middleware.TestResponseWriter{})
		_, err := etcMulti.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}
		resp := rec.Msg()

		sort.Sort(rrSet(resp.Answer))
		sort.Sort(rrSet(resp.Ns))
		sort.Sort(rrSet(resp.Extra))

		if resp.Rcode != tc.Rcode {
			t.Errorf("rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
			t.Logf("%v\n", resp)
			continue
		}

		if len(resp.Answer) != len(tc.Answer) {
			t.Errorf("answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
			t.Logf("%v\n", resp)
			continue
		}
		if len(resp.Ns) != len(tc.Ns) {
			t.Errorf("authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
			t.Logf("%v\n", resp)
			continue
		}
		if len(resp.Extra) != len(tc.Extra) {
			t.Errorf("additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
			t.Logf("%v\n", resp)
			continue
		}

		if !checkSection(t, tc, Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !checkSection(t, tc, Ns, resp.Ns) {
			t.Logf("%v\n", resp)

		}
		if !checkSection(t, tc, Extra, resp.Extra) {
			t.Logf("%v\n", resp)
		}
	}
}

// Note the key is encoded as DNS name, while in "reality" it is a etcd path.
var servicesMulti = []*msg.Service{
	{Host: "dev.server1", Port: 8080, Key: "a.server1.dev.region1.skydns.test."},
	{Host: "dev.server1", Port: 8080, Key: "a.server1.dev.region1.miek.nl."},
	{Host: "dev.server1", Port: 8080, Key: "a.server1.dev.region1.example.org."},
}

var dnsTestCasesMulti = []dnsTestCase{
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("a.server1.dev.region1.skydns.test. 300 SRV 10 100 8080 dev.server1.")},
	},
	{
		Qname: "a.server1.dev.region1.miek.nl.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("a.server1.dev.region1.miek.nl. 300 SRV 10 100 8080 dev.server1.")},
	},
	{
		Qname: "a.server1.dev.region1.example.org.", Qtype: dns.TypeSRV, Rcode: dns.RcodeServerFailure,
	},
}

func handler() middleware.Handler {
	return middleware.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return dns.RcodeServerFailure, nil
	})
}
