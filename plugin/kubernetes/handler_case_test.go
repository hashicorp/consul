package kubernetes

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

var dnsPreserveCaseCases = []test.Case{
	// Negative response
	{
		Qname: "not-a-service.testns.svc.ClUsTeR.lOcAl.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("ClUsTeR.lOcAl.	5	IN	SOA	ns.dns.ClUsTeR.lOcAl. hostmaster.ClUsTeR.lOcAl. 1499347823 7200 1800 86400 5"),
		},
	},
	// A Service
	{
		Qname: "SvC1.TeStNs.SvC.cLuStEr.LoCaL.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("SvC1.TeStNs.SvC.cLuStEr.LoCaL.	5	IN	A	10.0.0.1"),
		},
	},
	// SRV Service
	{
		Qname: "_HtTp._TcP.sVc1.TeStNs.SvC.cLuStEr.LoCaL.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_HtTp._TcP.sVc1.TeStNs.SvC.cLuStEr.LoCaL.	5	IN	SRV	0 100 80 svc1.testns.svc.cLuStEr.LoCaL."),
		},
		Extra: []dns.RR{
			test.A("svc1.testns.svc.cLuStEr.LoCaL.	5	IN	A	10.0.0.1"),
		},
	},
}

func TestPreserveCase(t *testing.T) {
	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}
	k.opts.ignoreEmptyService = true
	k.Next = test.NextHandler(dns.RcodeSuccess, nil)
	ctx := context.TODO()

	for i, tc := range dnsPreserveCaseCases {
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
