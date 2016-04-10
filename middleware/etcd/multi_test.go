// +build etcd

package etcd

// etcd needs to be running on http://127.0.0.1:2379
// *and* needs connectivity to the internet for remotely resolving names.

import (
	"sort"
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	coretest "github.com/miekg/coredns/middleware/testing"

	"github.com/miekg/dns"
)

func TestMultiLookup(t *testing.T) {
	etcMulti := etc
	etcMulti.Zones = []string{"skydns.test.", "miek.nl."}
	etcMulti.Next = coretest.ErrorHandler()

	for _, serv := range servicesMulti {
		set(t, etcMulti, serv.Key, 0, serv)
		defer delete(t, etcMulti, serv.Key)
	}
	for _, tc := range dnsTestCasesMulti {
		m := tc.Msg()

		rec := middleware.NewResponseRecorder(&coretest.ResponseWriter{})
		_, err := etcMulti.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}
		resp := rec.Msg()

		sort.Sort(coretest.RRSet(resp.Answer))
		sort.Sort(coretest.RRSet(resp.Ns))
		sort.Sort(coretest.RRSet(resp.Extra))

		if !coretest.Header(t, tc, resp) {
			t.Logf("%v\n", resp)
			continue
		}
		if !coretest.Section(t, tc, coretest.Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !coretest.Section(t, tc, coretest.Ns, resp.Ns) {
			t.Logf("%v\n", resp)
		}
		if !coretest.Section(t, tc, coretest.Extra, resp.Extra) {
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

var dnsTestCasesMulti = []coretest.Case{
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{coretest.SRV("a.server1.dev.region1.skydns.test. 300 SRV 10 100 8080 dev.server1.")},
	},
	{
		Qname: "a.server1.dev.region1.miek.nl.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{coretest.SRV("a.server1.dev.region1.miek.nl. 300 SRV 10 100 8080 dev.server1.")},
	},
	{
		Qname: "a.server1.dev.region1.example.org.", Qtype: dns.TypeSRV, Rcode: dns.RcodeServerFailure,
	},
}
