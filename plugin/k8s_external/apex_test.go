package external

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/kubernetes"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestApex(t *testing.T) {
	k := kubernetes.New([]string{"cluster.local."})
	k.Namespaces = map[string]struct{}{"testns": {}}
	k.APIConn = &external{}

	e := New()
	e.Zones = []string{"example.com."}
	e.Next = test.NextHandler(dns.RcodeSuccess, nil)
	e.externalFunc = k.External
	e.externalAddrFunc = externalAddress // internal test function

	ctx := context.TODO()
	for i, tc := range testsApex {
		r := tc.Msg()
		w := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := e.ServeDNS(ctx, w, r)
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

var testsApex = []test.Case{
	{
		Qname: "example.com.", Qtype: dns.TypeSOA, Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SOA("example.com.	5	IN	SOA	ns1.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 5"),
		},
	},
	{
		Qname: "example.com.", Qtype: dns.TypeNS, Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.NS("example.com.	5	IN	NS	ns1.dns.example.com."),
		},
		Extra: []dns.RR{
			test.A("ns1.dns.example.com.	5	IN	A	127.0.0.1"),
		},
	},
	{
		Qname: "example.com.", Qtype: dns.TypeSRV, Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	5	IN	SOA	ns1.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 5"),
		},
	},
	{
		Qname: "dns.example.com.", Qtype: dns.TypeSRV, Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	5	IN	SOA	ns1.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 5"),
		},
	},
	{
		Qname: "dns.example.com.", Qtype: dns.TypeNS, Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	5	IN	SOA	ns1.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 5"),
		},
	},
	{
		Qname: "ns1.dns.example.com.", Qtype: dns.TypeSRV, Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	5	IN	SOA	ns1.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 5"),
		},
	},
	{
		Qname: "ns1.dns.example.com.", Qtype: dns.TypeNS, Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	5	IN	SOA	ns1.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 5"),
		},
	},
	{
		Qname: "ns1.dns.example.com.", Qtype: dns.TypeAAAA, Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	5	IN	SOA	ns1.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 5"),
		},
	},
	{
		Qname: "ns1.dns.example.com.", Qtype: dns.TypeA, Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("ns1.dns.example.com.	5	IN	A	127.0.0.1"),
		},
	},
}
