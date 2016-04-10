// +build etcd

package etcd

// etcd needs to be running on http://127.0.0.1:2379

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	coretest "github.com/miekg/coredns/middleware/testing"

	"github.com/miekg/dns"
)

// Check the ordering of returned cname.
func TestCnameLookup(t *testing.T) {
	for _, serv := range servicesCname {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCasesCname {
		m := tc.Msg()

		rec := middleware.NewResponseRecorder(&coretest.ResponseWriter{})
		_, err := etc.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}
		resp := rec.Msg()

		if !coretest.Header(t, tc, resp) {
			t.Logf("%v\n", resp)
			continue
		}
		if !coretest.Section(t, tc, coretest.Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !coretest.Section(t, tc, coretest.Ns, resp.Ns) {
			t.Logf("%v\n", resp)
		}
		if !coretest.Section(t, tc, coretest.Extra, resp.Extra) {
			t.Logf("%v\n", resp)
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
	{Host: "10.240.0.1", Key: "endpoint.region2.skydns.test."},
}

var dnsTestCasesCname = []coretest.Case{
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			coretest.SRV("a.server1.dev.region1.skydns.test.	300	IN	SRV	10 100 0 cname1.region2.skydns.test."),
		},
		Extra: []dns.RR{
			coretest.CNAME("cname1.region2.skydns.test.	300	IN	CNAME	cname2.region2.skydns.test."),
			coretest.CNAME("cname2.region2.skydns.test.	300	IN	CNAME	cname3.region2.skydns.test."),
			coretest.CNAME("cname3.region2.skydns.test.	300	IN	CNAME	cname4.region2.skydns.test."),
			coretest.CNAME("cname4.region2.skydns.test.	300	IN	CNAME	cname5.region2.skydns.test."),
			coretest.CNAME("cname5.region2.skydns.test.	300	IN	CNAME	cname6.region2.skydns.test."),
			coretest.CNAME("cname6.region2.skydns.test.	300	IN	CNAME	endpoint.region2.skydns.test."),
			coretest.A("endpoint.region2.skydns.test.	300	IN	A	10.240.0.1"),
		},
	},
}
