// +build etcd

package etcd

import (
	"testing"

	"github.com/coredns/coredns/middleware/etcd/msg"
	"github.com/coredns/coredns/middleware/pkg/dnsrecorder"
	"github.com/coredns/coredns/middleware/proxy"
	"github.com/coredns/coredns/middleware/test"

	"github.com/miekg/dns"
)

func TestProxyLookupFailDebug(t *testing.T) {
	etc := newEtcdMiddleware()
	etc.Proxy = proxy.NewLookup([]string{"127.0.0.1:154"})
	etc.Debugging = true

	for _, serv := range servicesProxy {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}

	for _, tc := range dnsTestCasesProxy {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			continue
		}

		resp := rec.Msg
		test.SortAndCheck(t, resp, tc)
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
