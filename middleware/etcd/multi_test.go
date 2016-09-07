// +build etcd

package etcd

import (
	"sort"
	"testing"

	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
)

func TestMultiLookup(t *testing.T) {
	etc := newEtcdMiddleware()
	etc.Zones = []string{"skydns.test.", "miek.nl."}
	etc.Next = test.ErrorHandler()

	for _, serv := range servicesMulti {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCasesMulti {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}

		resp := rec.Msg
		sort.Sort(test.RRSet(resp.Answer))
		sort.Sort(test.RRSet(resp.Ns))
		sort.Sort(test.RRSet(resp.Extra))

		if !test.Header(t, tc, resp) {
			t.Logf("%v\n", resp)
			continue
		}
		if !test.Section(t, tc, test.Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !test.Section(t, tc, test.Ns, resp.Ns) {
			t.Logf("%v\n", resp)
		}
		if !test.Section(t, tc, test.Extra, resp.Extra) {
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

var dnsTestCasesMulti = []test.Case{
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{test.SRV("a.server1.dev.region1.skydns.test. 300 SRV 10 100 8080 dev.server1.")},
	},
	{
		Qname: "a.server1.dev.region1.miek.nl.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{test.SRV("a.server1.dev.region1.miek.nl. 300 SRV 10 100 8080 dev.server1.")},
	},
	{
		Qname: "a.server1.dev.region1.example.org.", Qtype: dns.TypeSRV, Rcode: dns.RcodeServerFailure,
	},
}
