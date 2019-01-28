// +build etcd

package etcd

// etcd needs to be running on http://localhost:2379

import (
	"testing"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

// Check the ordering of returned cname.
func TestCnameLookup(t *testing.T) {
	etc := newEtcdPlugin()

	for _, serv := range servicesCname {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCasesCname {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		if err := test.Header(tc, resp); err != nil {
			t.Error(err)
			continue
		}
		if err := test.Section(tc, test.Answer, resp.Answer); err != nil {
			t.Error(err)
		}
		if err := test.Section(tc, test.Ns, resp.Ns); err != nil {
			t.Error(err)
		}
		if err := test.Section(tc, test.Extra, resp.Extra); err != nil {
			t.Error(err)
		}
	}
}

var servicesCname = []*msg.Service{
	{Host: "cname1.region2.skydns.test", Key: "a.server1.dev.region1.skydns.test."},
	{Host: "cname2.region2.skydns.test", Key: "cname1.region2.skydns.test."},
	{Host: "cname3.region2.skydns.test", Key: "cname2.region2.skydns.test."},
	{Host: "cname4.region2.skydns.test", Key: "cname3.region2.skydns.test."},
	{Host: "cname5.region2.skydns.test", Key: "cname4.region2.skydns.test."},
	{Host: "cname6.region2.skydns.test", Key: "cname5.region2.skydns.test."},
	{Host: "endpoint.region2.skydns.test", Key: "cname6.region2.skydns.test."},
	{Host: "mainendpoint.region2.skydns.test", Key: "region2.skydns.test."},
	{Host: "", Key: "region3.skydns.test.", Text: "SOME-RECORD-TEXT"},
	{Host: "10.240.0.1", Key: "endpoint.region2.skydns.test."},
}

var dnsTestCasesCname = []test.Case{
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			test.SRV("a.server1.dev.region1.skydns.test.	300	IN	SRV	10 100 0 cname1.region2.skydns.test."),
		},
		Extra: []dns.RR{
			test.CNAME("cname1.region2.skydns.test.	300	IN	CNAME	cname2.region2.skydns.test."),
			test.CNAME("cname2.region2.skydns.test.	300	IN	CNAME	cname3.region2.skydns.test."),
			test.CNAME("cname3.region2.skydns.test.	300	IN	CNAME	cname4.region2.skydns.test."),
			test.CNAME("cname4.region2.skydns.test.	300	IN	CNAME	cname5.region2.skydns.test."),
			test.CNAME("cname5.region2.skydns.test.	300	IN	CNAME	cname6.region2.skydns.test."),
			test.CNAME("cname6.region2.skydns.test.	300	IN	CNAME	endpoint.region2.skydns.test."),
			test.A("endpoint.region2.skydns.test.	300	IN	A	10.240.0.1"),
		},
	},
	{
		Qname: "region2.skydns.test.", Qtype: dns.TypeCNAME,
		Answer: []dns.RR{
			test.CNAME("region2.skydns.test.	300	IN	CNAME	mainendpoint.region2.skydns.test."),
		},
	},
	{
		Qname: "region3.skydns.test.", Qtype: dns.TypeCNAME,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("skydns.test.	303	IN	SOA	ns.dns.skydns.test. hostmaster.skydns.test. 1546424605 7200 1800 86400 30"),
		},
	},
}
