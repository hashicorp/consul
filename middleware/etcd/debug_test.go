// +build etcd

package etcd

import (
	"sort"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
)

func TestIsDebug(t *testing.T) {
	if ok := isDebug("o-o.debug.miek.nl."); ok != "miek.nl." {
		t.Errorf("expected o-o.debug.miek.nl. to be debug")
	}
	if ok := isDebug(strings.ToLower("o-o.Debug.miek.nl.")); ok != "miek.nl." {
		t.Errorf("expected o-o.Debug.miek.nl. to be debug")
	}
	if ok := isDebug("i-o.debug.miek.nl."); ok != "" {
		t.Errorf("expected i-o.Debug.miek.nl. to be non-debug")
	}
	if ok := isDebug(strings.ToLower("i-o.Debug.")); ok != "" {
		t.Errorf("expected o-o.Debug. to be non-debug")
	}
}

func TestDebugLookup(t *testing.T) {
	etc := newEtcdMiddleware()
	etc.Debugging = true

	for _, serv := range servicesDebug {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}

	for _, tc := range dnsTestCasesDebug {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			continue
		}

		resp := rec.Msg
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

func TestDebugLookupFalse(t *testing.T) {
	etc := newEtcdMiddleware()

	for _, serv := range servicesDebug {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCasesDebugFalse {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			continue
		}

		resp := rec.Msg
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

var servicesDebug = []*msg.Service{
	{Host: "127.0.0.1", Key: "a.dom.skydns.test."},
	{Host: "127.0.0.2", Key: "b.sub.dom.skydns.test."},
}

var dnsTestCasesDebug = []test.Case{
	{
		Qname: "o-o.debug.dom.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("dom.skydns.test. 300 IN A 127.0.0.1"),
			test.A("dom.skydns.test. 300 IN A 127.0.0.2"),
		},
		Extra: []dns.RR{
			test.TXT(`a.dom.skydns.test. 300	CH	TXT	"127.0.0.1:0(10,0,,false)[0,]"`),
			test.TXT(`b.sub.dom.skydns.test. 300	CH	TXT	"127.0.0.2:0(10,0,,false)[0,]"`),
		},
	},
	{
		Qname: "o-o.debug.dom.skydns.test.", Qtype: dns.TypeTXT,
		Ns: []dns.RR{
			test.SOA("skydns.test. 300 IN SOA ns.dns.skydns.test. hostmaster.skydns.test. 1463943291 7200 1800 86400 60"),
		},
		Extra: []dns.RR{
			test.TXT(`a.dom.skydns.test. 300	CH	TXT	"127.0.0.1:0(10,0,,false)[0,]"`),
			test.TXT(`b.sub.dom.skydns.test. 300	CH	TXT	"127.0.0.2:0(10,0,,false)[0,]"`),
		},
	},
}

var dnsTestCasesDebugFalse = []test.Case{
	{
		Qname: "o-o.debug.dom.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("skydns.test. 300 IN SOA ns.dns.skydns.test. hostmaster.skydns.test. 1463943291 7200 1800 86400 60"),
		},
	},
	{
		Qname: "o-o.debug.dom.skydns.test.", Qtype: dns.TypeTXT,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("skydns.test. 300 IN SOA ns.dns.skydns.test. hostmaster.skydns.test. 1463943291 7200 1800 86400 60"),
		},
	},
}
