// +build etcd

// tests mx and txt records

package etcd

// etcd needs to be running on http://127.0.0.1:2379
// *and* needs connectivity to the internet for remotely resolving
// names.

import (
	"sort"
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"

	"github.com/miekg/dns"
)

func TestOtherLookup(t *testing.T) {
	for _, serv := range servicesOther {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCasesOther {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(tc.Qname), tc.Qtype)

		rec := middleware.NewResponseRecorder(&middleware.TestResponseWriter{})
		_, err := etc.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}
		resp := rec.Msg()

		sort.Sort(rrSet(resp.Answer))
		sort.Sort(rrSet(resp.Ns))
		sort.Sort(rrSet(resp.Extra))

		if resp.Rcode != tc.Rcode {
			t.Errorf("rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
			t.Logf("%v\n", resp)
			continue
		}

		if len(resp.Answer) != len(tc.Answer) {
			t.Errorf("answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
			t.Logf("%v\n", resp)
			continue
		}
		if len(resp.Ns) != len(tc.Ns) {
			t.Errorf("authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
			t.Logf("%v\n", resp)
			continue
		}
		if len(resp.Extra) != len(tc.Extra) {
			t.Errorf("additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
			t.Logf("%v\n", resp)
			continue
		}

		if !checkSection(t, tc, Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !checkSection(t, tc, Ns, resp.Ns) {
			t.Logf("%v\n", resp)

		}
		if !checkSection(t, tc, Extra, resp.Extra) {
			t.Logf("%v\n", resp)
		}
	}
}

// Note the key is encoded as DNS name, while in "reality" it is a etcd path.
var servicesOther = []*msg.Service{
	{Host: "dev.server1", Port: 8080, Key: "a.server1.dev.region1.skydns.test."},

	// mx
	{Host: "mx.skydns.test", Priority: 50, Mail: true, Key: "a.mail.skydns.test."},
	{Host: "mx.miek.nl", Priority: 50, Mail: true, Key: "b.mail.skydns.test."},
	{Host: "a.ipaddr.skydns.test", Priority: 30, Mail: true, Key: "a.mx.skydns.test."},

	{Host: "a.ipaddr.skydns.test", Mail: true, Key: "a.mx2.skydns.test."},
	{Host: "b.ipaddr.skydns.test", Mail: true, Key: "b.mx2.skydns.test."},

	{Host: "172.16.1.1", Key: "a.ipaddr.skydns.test."},
	{Host: "172.16.1.2", Key: "b.ipaddr.skydns.test."},

	// txt
	{Text: "abc", Key: "a1.txt.skydns.test."},
	{Text: "abc abc", Key: "a2.txt.skydns.test."},

	// duplicate ip address
	{Host: "10.11.11.10", Key: "http.multiport.http.skydns.test.", Port: 80},
	{Host: "10.11.11.10", Key: "https.multiport.http.skydns.test.", Port: 443},
}

var dnsTestCasesOther = []dnsTestCase{
	// MX Tests
	{
		// NODATA as this is not an Mail: true record.
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeMX,
		Ns: []dns.RR{
			newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0"),
		},
	},
	{
		Qname: "a.mail.skydns.test.", Qtype: dns.TypeMX,
		Answer: []dns.RR{newMX("a.mail.skydns.test. 300 IN MX 50 mx.skydns.test.")},
		Extra: []dns.RR{
			newA("a.ipaddr.skydns.test.	300	IN	A	172.16.1.1"),
			newCNAME("mx.skydns.test.	300	IN	CNAME	a.ipaddr.skydns.test."),
		},
	},
	{
		Qname: "mx2.skydns.test.", Qtype: dns.TypeMX,
		Answer: []dns.RR{
			newMX("mx2.skydns.test. 300 IN MX 10 a.ipaddr.skydns.test."),
			newMX("mx2.skydns.test. 300 IN MX 10 b.ipaddr.skydns.test."),
		},
		Extra: []dns.RR{
			newA("a.ipaddr.skydns.test. 300 A 172.16.1.1"),
			newA("b.ipaddr.skydns.test. 300 A 172.16.1.2"),
		},
	},
	// Txt
	{
		Qname: "a1.txt.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			newTXT("a1.txt.skydns.test. 300 IN TXT \"abc\""),
		},
	},
	{
		Qname: "a2.txt.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			newTXT("a2.txt.skydns.test. 300 IN TXT \"abc abc\""),
		},
	},
	{
		Qname: "txt.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			newTXT("txt.skydns.test. 300 IN TXT \"abc abc\""),
			newTXT("txt.skydns.test. 300 IN TXT \"abc\""),
		},
	},
	// Duplicate IP address test
	{
		Qname: "multiport.http.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{newA("multiport.http.skydns.test. 300 IN A 10.11.11.10")},
	},
}
