// +build etcd

package etcd

import (
	"sort"
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
)

func TestProxyLookupFailDebug(t *testing.T) {
	for _, serv := range servicesProxy {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}

	prxy := etc.Proxy
	etc.Proxy = proxy.New([]string{"127.0.0.0:154"})
	etc.Debug = true

	defer func() { etc.Debug = false }()
	defer func() { etc.Proxy = prxy }()

	for _, tc := range dnsTestCasesProxy {
		m := tc.Msg()

		rec := middleware.NewResponseRecorder(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
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

// Note the key is encoded as DNS name, while in "reality" it is a etcd path.
var servicesProxy = []*msg.Service{
	{Host: "www.example.org", Key: "a.dom.skydns.test."},
}

var dnsTestCasesProxy = []test.Case{
	{
		Qname: "dom.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			test.SRV("dom.skydns.test. 300 IN SRV 10 100 0 www.example.org."),
		},
		Extra: []dns.RR{
			test.TXT(".	0	CH	TXT	\"www.example.org. IN A: unreachable backend\""),
			test.TXT(".	0	CH	TXT	\"www.example.org. IN AAAA: unreachable backend\""),
		},
	},
}
