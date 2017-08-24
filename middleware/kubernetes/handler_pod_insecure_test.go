package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/middleware/pkg/dnsrecorder"
	"github.com/coredns/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var podModeInsecureCases = map[string](test.Case){

	"A Record Pod mode = Case 1": {
		Qname: "10-240-0-1.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("10-240-0-1.podns.pod.cluster.local.	0	IN	A	10.240.0.1"),
		},
	},

	"A Record Pod mode = Case 2": {
		Qname: "172-0-0-2.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("172-0-0-2.podns.pod.cluster.local.	0	IN	A	172.0.0.2"),
		},
	},
}

func TestServeDNSModeInsecure(t *testing.T) {

	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}
	k.Next = test.NextHandler(dns.RcodeSuccess, nil)
	ctx := context.TODO()
	k.podMode = podModeInsecure

	for testname, tc := range podModeInsecureCases {
		r := tc.Msg()

		w := dnsrecorder.New(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("%v expected no error, got %v\n", testname, err)
			return
		}
		if tc.Error != nil {
			continue
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("got nil message and no error for %q: %s %d", testname, r.Question[0].Name, r.Question[0].Qtype)
		}

		test.SortAndCheck(t, resp, tc)
	}
}
