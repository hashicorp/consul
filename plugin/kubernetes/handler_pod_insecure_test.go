package kubernetes

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

var podModeInsecureCases = []test.Case{
	{
		Qname: "10-240-0-1.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("10-240-0-1.podns.pod.cluster.local.	5	IN	A	10.240.0.1"),
		},
	},
	{
		Qname: "172-0-0-2.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("172-0-0-2.podns.pod.cluster.local.	5	IN	A	172.0.0.2"),
		},
	},
	{
		Qname: "blah.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1515173576 7200 1800 86400 30"),
		},
	},
	{
		Qname: "blah.podns.pod.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1515173576 7200 1800 86400 30"),
		},
	},
	{
		Qname: "blah.podns.pod.cluster.local.", Qtype: dns.TypeHINFO,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1515173576 7200 1800 86400 30"),
		},
	},
	{
		Qname: "blah.pod-nons.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1515173576 7200 1800 86400 30"),
		},
	},
	{
		Qname: "podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1515173576 7200 1800 86400 30"),
		},
	},
}

func TestServeDNSModeInsecure(t *testing.T) {

	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}
	k.Next = test.NextHandler(dns.RcodeSuccess, nil)
	ctx := context.TODO()
	k.podMode = podModeInsecure

	for i, tc := range podModeInsecureCases {
		r := tc.Msg()

		w := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("Test %d expected no error, got %v", i, err)
			return
		}
		if tc.Error != nil {
			continue
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("Test %d, got nil message and no error for %q", i, r.Question[0].Name)
		}

		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
		}
	}
}
