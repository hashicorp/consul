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
	defer func() { etc.Proxy = prxy }()

	etc.Debug = true
	defer func() { etc.Debug = false }()

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

var servicesProxy = []*msg.Service{
	{Host: "www.example.org", Key: "a.dom.skydns.test."},
}

var dnsTestCasesProxy = []test.Case{
	{
		Qname: "o-o.debug.dom.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			test.SRV("dom.skydns.test. 300 IN SRV 10 100 0 www.example.org."),
		},
		Extra: []dns.RR{
			test.TXT("a.dom.skydns.test.	300	CH	TXT	\"www.example.org:0(10,0,,false)[0,]\""),
			test.TXT("www.example.org.	0	CH	TXT	\"www.example.org.:0(0,0, IN A: unreachable backend,false)[0,]\""),
			test.TXT("www.example.org.	0	CH	TXT	\"www.example.org.:0(0,0, IN AAAA: unreachable backend,false)[0,]\""),
		},
	},
}
