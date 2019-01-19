// +build etcd

// tests mx and txt records

package etcd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestOtherLookup(t *testing.T) {
	etc := newEtcdPlugin()

	for _, serv := range servicesOther {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCasesOther {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			continue
		}

		resp := rec.Msg
		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
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

	{Host: "a.ipaddr.skydns.test", Priority: 20, Mail: true, Key: "a.mx3.skydns.test."},
	{Host: "a.ipaddr.skydns.test", Priority: 30, Mail: true, Key: "b.mx3.skydns.test."},

	{Host: "172.16.1.1", Key: "a.ipaddr.skydns.test."},
	{Host: "172.16.1.2", Key: "b.ipaddr.skydns.test."},

	// txt
	{Text: "abc", Key: "a1.txt.skydns.test."},
	{Text: "abc abc", Key: "a2.txt.skydns.test."},
	// txt sizes
	{Text: strings.Repeat("0", 400), Key: "large400.skydns.test."},
	{Text: strings.Repeat("0", 600), Key: "large600.skydns.test."},
	{Text: strings.Repeat("0", 2000), Key: "large2000.skydns.test."},

	// duplicate ip address
	{Host: "10.11.11.10", Key: "http.multiport.http.skydns.test.", Port: 80},
	{Host: "10.11.11.10", Key: "https.multiport.http.skydns.test.", Port: 443},
}

var dnsTestCasesOther = []test.Case{
	// MX Tests
	{
		// NODATA as this is not an Mail: true record.
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeMX,
		Ns: []dns.RR{
			test.SOA("skydns.test. 30 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0"),
		},
	},
	{
		Qname: "a.mail.skydns.test.", Qtype: dns.TypeMX,
		Answer: []dns.RR{test.MX("a.mail.skydns.test. 300 IN MX 50 mx.skydns.test.")},
		Extra: []dns.RR{
			test.A("a.ipaddr.skydns.test.	300	IN	A	172.16.1.1"),
			test.CNAME("mx.skydns.test.	300	IN	CNAME	a.ipaddr.skydns.test."),
		},
	},
	{
		Qname: "mx2.skydns.test.", Qtype: dns.TypeMX,
		Answer: []dns.RR{
			test.MX("mx2.skydns.test. 300 IN MX 10 a.ipaddr.skydns.test."),
			test.MX("mx2.skydns.test. 300 IN MX 10 b.ipaddr.skydns.test."),
		},
		Extra: []dns.RR{
			test.A("a.ipaddr.skydns.test. 300 A 172.16.1.1"),
			test.A("b.ipaddr.skydns.test. 300 A 172.16.1.2"),
		},
	},
	// different priority, same host
	{
		Qname: "mx3.skydns.test.", Qtype: dns.TypeMX,
		Answer: []dns.RR{
			test.MX("mx3.skydns.test. 300 IN MX 20 a.ipaddr.skydns.test."),
			test.MX("mx3.skydns.test. 300 IN MX 30 a.ipaddr.skydns.test."),
		},
		Extra: []dns.RR{
			test.A("a.ipaddr.skydns.test. 300 A 172.16.1.1"),
		},
	},
	// Txt
	{
		Qname: "a1.txt.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT("a1.txt.skydns.test. 300 IN TXT \"abc\""),
		},
	},
	{
		Qname: "a2.txt.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT("a2.txt.skydns.test. 300 IN TXT \"abc abc\""),
		},
	},
	// Large txt less than 512
	{
		Qname: "large400.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT(fmt.Sprintf("large400.skydns.test. 300 IN TXT \"%s\"", strings.Repeat("0", 400))),
		},
	},
	// Duplicate IP address test
	{
		Qname: "multiport.http.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{test.A("multiport.http.skydns.test. 300 IN A 10.11.11.10")},
	},
}
