// +build etcd

package etcd

import (
	"sort"
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/dns"
)

func TestStubLookup(t *testing.T) {
	for _, serv := range servicesStub {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	etc.updateStubZones()

	for _, tc := range dnsTestCasesStub {
		m := tc.Msg()

		rec := middleware.NewResponseRecorder(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			if tc.Rcode != dns.RcodeServerFailure {
				t.Errorf("expected no error, got %v\n", err)
			}
			// This is OK, we expect this backend to *not* work.
			continue
		}
		resp := rec.Msg()
		if resp == nil {
			// etcd not running?
			continue
		}

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

var servicesStub = []*msg.Service{
	// Two tests, ask a question that should return servfail because remote it no accessible
	// and one with edns0 option added, that should return refused.
	{Host: "127.0.0.1", Port: 666, Key: "b.example.org.stub.dns.skydns.test."},
	// Actual test that goes out to the internet.
	{Host: "199.43.132.53", Key: "a.example.net.stub.dns.skydns.test."},
}

var dnsTestCasesStub = []test.Case{
	{
		Qname: "example.org.", Qtype: dns.TypeA, Rcode: dns.RcodeServerFailure,
	},
	{
		Qname: "example.net.", Qtype: dns.TypeA,
		Answer: []dns.RR{test.A("example.net.	86400	IN	A	93.184.216.34")},
		Ns: []dns.RR{
			test.NS("example.net.	86400	IN	NS	a.iana-servers.net."),
			test.NS("example.net.	86400	IN	NS	b.iana-servers.net."),
		},
		Extra: []dns.RR{test.OPT(4096, false)}, // This will have an EDNS0 section, because *we* added our local stub forward to detect loops.
	},
	{
		Qname: "example.net.", Qtype: dns.TypeA, Do: true,
		Answer: []dns.RR{
			test.A("example.net.	86400	IN	A	93.184.216.34"),
			test.RRSIG("example.net.	86400	IN	RRSIG	A 8 2 86400 20160428060557 20160406182909 40948 example.net. Vm+rH5KN"),
		},
		Ns: []dns.RR{
			test.NS("example.net.	86400	IN	NS	a.iana-servers.net."),
			test.NS("example.net.	86400	IN	NS	b.iana-servers.net."),
			test.RRSIG("example.net.	86400	IN	RRSIG	NS 8 2 86400 20160428110538 20160407002909 40948 example.net. z74YR2"),
		},
		Extra: []dns.RR{test.OPT(4096, true)},
	},
}
