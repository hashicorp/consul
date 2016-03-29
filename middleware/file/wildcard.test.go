package file

import (
	"sort"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware"
	coretest "github.com/miekg/coredns/middleware/testing"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var dnssecWildcardTestCases = []coretest.Case{
	{
		Qname: "miek.nl.", Qtype: dns.TypeSOA, Do: true,
		Answer: []dns.RR{
			// because we sort, this look fishy, but it is OK.
			coretest.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. FIrzy07acBbtyQczy1dc="),
			coretest.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeAAAA, Do: true,
		Answer: []dns.RR{
			coretest.AAAA("miek.nl.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fef1:6735"),
			coretest.RRSIG("miek.nl.	1800	IN	RRSIG	AAAA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. SsRT="),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeMX, Do: true,
		Answer: []dns.RR{
			coretest.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com."),
			coretest.MX("miek.nl.	1800	IN	MX	10 aspmx2.googlemail.com."),
			coretest.MX("miek.nl.	1800	IN	MX	10 aspmx3.googlemail.com."),
			coretest.MX("miek.nl.	1800	IN	MX	5 alt1.aspmx.l.google.com."),
			coretest.MX("miek.nl.	1800	IN	MX	5 alt2.aspmx.l.google.com."),
			coretest.RRSIG("miek.nl.	1800	IN	RRSIG	MX 8 2 1800 20160426031301 20160327031301 12051 miek.nl. kLqG+iOr="),
		},
	},
	{
		Qname: "www.miek.nl.", Qtype: dns.TypeA, Do: true,
		Answer: []dns.RR{
			coretest.CNAME("www.miek.nl.	1800	IN	CNAME	a.miek.nl."),
		},

		Extra: []dns.RR{
			coretest.A("a.miek.nl.	1800	IN	A	139.162.196.78"),
			coretest.RRSIG("a.miek.nl.	1800	IN	RRSIG	A 8 3 1800 20160426031301 20160327031301 12051 miek.nl. lxLotCjWZ3kihTxk="),
		},
	},
	{
		// NoData
		Qname: "a.miek.nl.", Qtype: dns.TypeSRV, Do: true,
		Ns: []dns.RR{
			coretest.NSEC("a.miek.nl.	14400	IN	NSEC	archive.miek.nl. A AAAA RRSIG NSEC"),
			coretest.RRSIG("a.miek.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160426031301 20160327031301 12051 miek.nl. GqnF6cutipmSHEao="),
			coretest.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. FIrzy07acBbtyQczy1dc="),
			coretest.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	/* HAHA nsec... shit.
	// disprove *.miek.nl and that b.miek.nl does not exist
	{
		Qname: "b.miek.nl.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			coretest.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	*/
}

func testLookupDNSSECWildcard(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbMiekNL_signed), testzone, "stdin")
	if err != nil {
		t.Fatalf("expect no error when reading zone, got %q", err)
	}

	fm := File{Next: coretest.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range dnssecWildcardTestCases {
		m := tc.Msg()

		rec := middleware.NewResponseRecorder(&middleware.TestResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}
		resp := rec.Msg()

		sort.Sort(coretest.RRSet(resp.Answer))
		sort.Sort(coretest.RRSet(resp.Ns))
		sort.Sort(coretest.RRSet(resp.Extra))

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

		if !coretest.CheckSection(t, tc, coretest.Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !coretest.CheckSection(t, tc, coretest.Ns, resp.Ns) {
			t.Logf("%v\n", resp)

		}
		if !coretest.CheckSection(t, tc, coretest.Extra, resp.Extra) {
			t.Logf("%v\n", resp)
		}
	}
}

const dbMiekNL_wildcard_signed = `§`
