package dnssec

import (
	"sort"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware/file"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var dnssecTestCases = []test.Case{
	{
		Qname: "miek.nl.", Qtype: dns.TypeDNSKEY,
		Answer: []dns.RR{
			test.DNSKEY("miek.nl.	3600	IN	DNSKEY	257 3 13 0J8u0XJ9GNGFEBXuAmLu04taHG4"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeDNSKEY, Do: true,
		Answer: []dns.RR{
			test.DNSKEY("miek.nl.	3600	IN	DNSKEY	257 3 13 0J8u0XJ9GNGFEBXuAmLu04taHG4"),
			test.RRSIG("miek.nl.	3600	IN	RRSIG	DNSKEY 13 2 3600 20160503150844 20160425120844 18512 miek.nl. Iw/kNOyM"),
		},
		Extra: []dns.RR{test.OPT(4096, true)},
	},
}

var dnsTestCases = []test.Case{
	{
		Qname: "miek.nl.", Qtype: dns.TypeDNSKEY,
		Answer: []dns.RR{
			test.DNSKEY("miek.nl.	3600	IN	DNSKEY	257 3 13 0J8u0XJ9GNGFEBXuAmLu04taHG4"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeMX,
		Answer: []dns.RR{
			test.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com."),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeMX, Do: true,
		Answer: []dns.RR{
			test.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com."),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	MX 13 2 3600 20160503192428 20160425162428 18512 miek.nl. 4nxuGKitXjPVA9zP1JIUvA09"),
		},
		Extra: []dns.RR{test.OPT(4096, true)},
	},
	{
		Qname: "www.miek.nl.", Qtype: dns.TypeAAAA, Do: true,
		Answer: []dns.RR{
			test.AAAA("a.miek.nl.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fef1:6735"),
			test.RRSIG("a.miek.nl.	1800	IN	RRSIG	AAAA 13 3 3600 20160503193047 20160425163047 18512 miek.nl. UAyMG+gcnoXW3"),
			test.CNAME("www.miek.nl.	1800	IN	CNAME	a.miek.nl."),
			test.RRSIG("www.miek.nl.	1800	IN	RRSIG	CNAME 13 3 3600 20160503193047 20160425163047 18512 miek.nl. E3qGZn"),
		},
		Extra: []dns.RR{test.OPT(4096, true)},
	},
	{
		Qname: "www.example.org.", Qtype: dns.TypeAAAA, Do: true,
		Rcode: dns.RcodeServerFailure,
		// Extra: []dns.RR{test.OPT(4096, true)}, // test.ErrorHandler is a simple handler that does not do EDNS.
	},
}

func TestLookupZone(t *testing.T) {
	zone, err := file.Parse(strings.NewReader(dbMiekNL), "miek.nl.", "stdin")
	if err != nil {
		return
	}
	fm := file.File{Next: test.ErrorHandler(), Zones: file.Zones{Z: map[string]*file.Zone{"miek.nl.": zone}, Names: []string{"miek.nl."}}}
	dnskey, rm1, rm2 := newKey(t)
	defer rm1()
	defer rm2()
	cache, _ := lru.New(defaultCap)
	dh := New([]string{"miek.nl."}, []*DNSKEY{dnskey}, fm, cache)
	ctx := context.TODO()

	for _, tc := range dnsTestCases {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := dh.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
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

func TestLookupDNSKEY(t *testing.T) {
	dnskey, rm1, rm2 := newKey(t)
	defer rm1()
	defer rm2()
	cache, _ := lru.New(defaultCap)
	dh := New([]string{"miek.nl."}, []*DNSKEY{dnskey}, test.ErrorHandler(), cache)
	ctx := context.TODO()

	for _, tc := range dnssecTestCases {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := dh.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}

		resp := rec.Msg
		if !resp.Authoritative {
			t.Errorf("Authoritative Answer should be true, got false")
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

const dbMiekNL = `
$TTL    30M
$ORIGIN miek.nl.
@       IN      SOA     linode.atoom.net. miek.miek.nl. (
                             1282630057 ; Serial
                             4H         ; Refresh
                             1H         ; Retry
                             7D         ; Expire
                             4H )       ; Negative Cache TTL
                IN      NS      linode.atoom.net.

                IN      MX      1  aspmx.l.google.com.

                IN      A       139.162.196.78
                IN      AAAA    2a01:7e00::f03c:91ff:fef1:6735

a               IN      A       139.162.196.78
                IN      AAAA    2a01:7e00::f03c:91ff:fef1:6735
www             IN      CNAME   a`
