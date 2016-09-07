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

func TestGroupLookup(t *testing.T) {
	etc := newEtcdMiddleware()

	for _, serv := range servicesGroup {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCasesGroup {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			continue
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
var servicesGroup = []*msg.Service{
	{Host: "127.0.0.1", Key: "a.dom.skydns.test.", Group: "g1"},
	{Host: "127.0.0.2", Key: "b.sub.dom.skydns.test.", Group: "g1"},

	{Host: "127.0.0.1", Key: "a.dom2.skydns.test.", Group: "g1"},
	{Host: "127.0.0.2", Key: "b.sub.dom2.skydns.test.", Group: ""},

	{Host: "127.0.0.1", Key: "a.dom1.skydns.test.", Group: "g1"},
	{Host: "127.0.0.2", Key: "b.sub.dom1.skydns.test.", Group: "g2"},
}

var dnsTestCasesGroup = []test.Case{
	// Groups
	{
		// hits the group 'g1' and only includes those records
		Qname: "dom.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("dom.skydns.test. 300 IN A 127.0.0.1"),
			test.A("dom.skydns.test. 300 IN A 127.0.0.2"),
		},
	},
	{
		// One has group, the other has not...  Include the non-group always.
		Qname: "dom2.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("dom2.skydns.test. 300 IN A 127.0.0.1"),
			test.A("dom2.skydns.test. 300 IN A 127.0.0.2"),
		},
	},
	{
		// The groups differ.
		Qname: "dom1.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("dom1.skydns.test. 300 IN A 127.0.0.1"),
		},
	},
}
