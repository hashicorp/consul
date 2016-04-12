// +build etcd

package etcd

import (
	"sort"
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/dns"
)

func TestStubLookup(t *testing.T) {
	// reuse servics from stub_test.go
	for _, serv := range servicesStub {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	etc.updateStubZones()

	for _, tc := range dnsTestCasesCycleStub {
		m := tc.Msg()
		if tc.Do {
			// add our wacky edns fluff
			m.Extra[0] = ednsStub
		}

		rec := middleware.NewResponseRecorder(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			continue
		}
		resp := rec.Msg()

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

var dnsTestCasesCycleStub = []test.Case{
	{
		Qname: "example.org.", Qtype: dns.TypeA, Rcode: dns.RcodeRefused, Do: true,
	},
}
