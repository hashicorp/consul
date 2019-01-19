// +build etcd

package etcd

import (
	"testing"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestMultiLookup(t *testing.T) {
	etc := newEtcdPlugin()
	etc.Zones = []string{"skydns.test.", "miek.nl."}
	etc.Next = test.ErrorHandler()

	for _, serv := range servicesMulti {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCasesMulti {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
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
