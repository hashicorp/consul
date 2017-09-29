package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var kubeApexCases = []test.Case{
	{
		Qname: "cluster.local.", Qtype: dns.TypeSOA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	{
		Qname: "cluster.local.", Qtype: dns.TypeHINFO,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	{
		Qname: "cluster.local.", Qtype: dns.TypeNS,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.NS("cluster.local. 303     IN      NS     ns.dns.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("ns.dns.cluster.local.   303       IN      A       127.0.0.1"),
		},
	},
}

func TestServeDNSApex(t *testing.T) {

	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}
	k.Next = test.NextHandler(dns.RcodeSuccess, nil)
	ctx := context.TODO()

	for i, tc := range kubeApexCases {
		r := tc.Msg()

		w := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("Test %d, expected no error, got %v\n", i, err)
			return
		}
		if tc.Error != nil {
			continue
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("Test %d, got nil message and no error ford", i)
		}

		test.SortAndCheck(t, resp, tc)
	}
}
