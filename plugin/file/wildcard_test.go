package file

import (
	"context"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

// these examples don't have an additional opt RR set, because that's gets added by the server.
var wildcardTestCases = []test.Case{
	{
		Qname: "wild.dnssex.nl.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT(`wild.dnssex.nl.	1800	IN	TXT	"Doing It Safe Is Better"`),
		},
		Ns: dnssexAuth[:len(dnssexAuth)-1], // remove RRSIG on the end
	},
	{
		Qname: "a.wild.dnssex.nl.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT(`a.wild.dnssex.nl.	1800	IN	TXT	"Doing It Safe Is Better"`),
		},
		Ns: dnssexAuth[:len(dnssexAuth)-1], // remove RRSIG on the end
	},
	{
		Qname: "wild.dnssex.nl.", Qtype: dns.TypeTXT, Do: true,
		Answer: []dns.RR{
			test.RRSIG("wild.dnssex.nl.	1800	IN	RRSIG	TXT 8 2 1800 20160428190224 20160329190224 14460 dnssex.nl. FUZSTyvZfeuuOpCm"),
			test.TXT(`wild.dnssex.nl.	1800	IN	TXT	"Doing It Safe Is Better"`),
		},
		Ns: append([]dns.RR{
			test.NSEC("a.dnssex.nl.	14400	IN	NSEC	www.dnssex.nl. A AAAA RRSIG NSEC"),
			test.RRSIG("a.dnssex.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160428190224 20160329190224 14460 dnssex.nl. S+UMs2ySgRaaRY"),
		}, dnssexAuth...),
	},
	{
		Qname: "a.wild.dnssex.nl.", Qtype: dns.TypeTXT, Do: true,
		Answer: []dns.RR{
			test.RRSIG("a.wild.dnssex.nl.	1800	IN	RRSIG	TXT 8 2 1800 20160428190224 20160329190224 14460 dnssex.nl. FUZSTyvZfeuuOpCm"),
			test.TXT(`a.wild.dnssex.nl.	1800	IN	TXT	"Doing It Safe Is Better"`),
		},
		Ns: append([]dns.RR{
			test.NSEC("a.dnssex.nl.	14400	IN	NSEC	www.dnssex.nl. A AAAA RRSIG NSEC"),
			test.RRSIG("a.dnssex.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160428190224 20160329190224 14460 dnssex.nl. S+UMs2ySgRaaRY"),
		}, dnssexAuth...),
	},
	// nodata responses
	{
		Qname: "wild.dnssex.nl.", Qtype: dns.TypeSRV,
		Ns: []dns.RR{
			test.SOA(`dnssex.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1459281744 14400 3600 604800 14400`),
		},
	},
	{
		Qname: "wild.dnssex.nl.", Qtype: dns.TypeSRV, Do: true,
		Ns: []dns.RR{
			// TODO(miek): needs closest encloser proof as well? This is the wrong answer
			test.NSEC(`*.dnssex.nl.	14400	IN	NSEC	a.dnssex.nl. TXT RRSIG NSEC`),
			test.RRSIG(`*.dnssex.nl.	14400	IN	RRSIG	NSEC 8 2 14400 20160428190224 20160329190224 14460 dnssex.nl. os6INm6q2eXknD5z8TaaDOV+Ge/Ko+2dXnKP+J1fqJzafXJVH1F0nDrcXmMlR6jlBHA=`),
			test.RRSIG(`dnssex.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160428190224 20160329190224 14460 dnssex.nl. CA/Y3m9hCOiKC/8ieSOv8SeP964Bq++lyH8BZJcTaabAsERs4xj5PRtcxicwQXZiF8fYUCpROlUS0YR8Cdw=`),
			test.SOA(`dnssex.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1459281744 14400 3600 604800 14400`),
		},
	},
}

var dnssexAuth = []dns.RR{
	test.NS("dnssex.nl.	1800	IN	NS	linode.atoom.net."),
	test.NS("dnssex.nl.	1800	IN	NS	ns-ext.nlnetlabs.nl."),
	test.NS("dnssex.nl.	1800	IN	NS	omval.tednet.nl."),
	test.RRSIG("dnssex.nl.	1800	IN	RRSIG	NS 8 2 1800 20160428190224 20160329190224 14460 dnssex.nl. dLIeEvP86jj5ndkcLzhgvWixTABjWAGRTGQsPsVDFXsGMf9TGGC9FEomgkCVeNC0="),
}

func TestLookupWildcard(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbDnssexNLSigned), testzone1, "stdin", 0)
	if err != nil {
		t.Fatalf("Expect no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone1: zone}, Names: []string{testzone1}}}
	ctx := context.TODO()

	for _, tc := range wildcardTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
		}
	}
}

var wildcardDoubleTestCases = []test.Case{
	{
		Qname: "wild.w.example.org.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT(`wild.w.example.org. IN	TXT	"Wildcard"`),
		},
		Ns: exampleAuth,
	},
	{
		Qname: "wild.c.example.org.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT(`wild.c.example.org. IN	TXT	"c Wildcard"`),
		},
		Ns: exampleAuth,
	},
	{
		Qname: "wild.d.example.org.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT(`alias.example.org. IN	TXT	"Wildcard CNAME expansion"`),
			test.CNAME(`wild.d.example.org. IN	CNAME	alias.example.org`),
		},
		Ns: exampleAuth,
	},
	{
		Qname: "alias.example.org.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			test.TXT(`alias.example.org. IN	TXT	"Wildcard CNAME expansion"`),
		},
		Ns: exampleAuth,
	},
}

var exampleAuth = []dns.RR{
	test.NS("example.org.	3600	IN	NS	a.iana-servers.net."),
	test.NS("example.org.	3600	IN	NS	b.iana-servers.net."),
}

func TestLookupDoubleWildcard(t *testing.T) {
	zone, err := Parse(strings.NewReader(exampleOrg), "example.org.", "stdin", 0)
	if err != nil {
		t.Fatalf("Expect no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{"example.org.": zone}, Names: []string{"example.org."}}}
	ctx := context.TODO()

	for _, tc := range wildcardDoubleTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
		}
	}
}

func TestReplaceWithAsteriskLabel(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{".", ""},
		{"miek.nl.", "*.nl."},
		{"www.miek.nl.", "*.miek.nl."},
	}

	for _, tc := range tests {
		got := replaceWithAsteriskLabel(tc.in)
		if got != tc.out {
			t.Errorf("Expected to be %s, got %s", tc.out, got)
		}
	}
}

var apexWildcardTestCases = []test.Case{
	{
		Qname: "foo.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{test.A(`foo.example.org. 3600	IN	A 127.0.0.54`)},
		Ns: []dns.RR{test.NS(`example.org. 3600 IN NS b.iana-servers.net.`)},
	},
	{
		Qname: "bar.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{test.A(`bar.example.org. 3600	IN	A 127.0.0.53`)},
		Ns: []dns.RR{test.NS(`example.org. 3600 IN NS b.iana-servers.net.`)},
	},
}

func TestLookupApexWildcard(t *testing.T) {
	const name = "example.org."
	zone, err := Parse(strings.NewReader(apexWildcard), name, "stdin", 0)
	if err != nil {
		t.Fatalf("Expect no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{name: zone}, Names: []string{name}}}
	ctx := context.TODO()

	for _, tc := range apexWildcardTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
		}
	}
}

var multiWildcardTestCases = []test.Case{
	{
		Qname: "foo.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{test.A(`foo.example.org. 3600	IN	A 127.0.0.54`)},
		Ns: []dns.RR{test.NS(`example.org. 3600 IN NS b.iana-servers.net.`)},
	},
	{
		Qname: "bar.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{test.A(`bar.example.org. 3600	IN	A 127.0.0.53`)},
		Ns: []dns.RR{test.NS(`example.org. 3600 IN NS b.iana-servers.net.`)},
	},
	{
		Qname: "bar.intern.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{test.A(`bar.intern.example.org. 3600	IN	A 127.0.1.52`)},
		Ns: []dns.RR{test.NS(`example.org. 3600 IN NS b.iana-servers.net.`)},
	},
}

func TestLookupMultiWildcard(t *testing.T) {
	const name = "example.org."
	zone, err := Parse(strings.NewReader(doubleWildcard), name, "stdin", 0)
	if err != nil {
		t.Fatalf("Expect no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{name: zone}, Names: []string{name}}}
	ctx := context.TODO()

	for _, tc := range multiWildcardTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
		}
	}
}

const exampleOrg = `; example.org test file
$TTL 3600
example.org.		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600
example.org.		IN	NS	b.iana-servers.net.
example.org.		IN	NS	a.iana-servers.net.
example.org.		IN	A	127.0.0.1
example.org.		IN	A	127.0.0.2
*.w.example.org.        IN      TXT     "Wildcard"
a.b.c.w.example.org.    IN      TXT     "Not a wildcard"
*.c.example.org.        IN      TXT     "c Wildcard"
*.d.example.org.        IN      CNAME   alias.example.org.
alias.example.org.      IN      TXT     "Wildcard CNAME expansion"
`

const apexWildcard = `; example.org test file with wildcard at apex
$TTL 3600
example.org.		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600
example.org.		IN	NS	b.iana-servers.net.
*.example.org.          IN      A       127.0.0.53
foo.example.org.        IN      A       127.0.0.54
`

const doubleWildcard = `; example.org test file with wildcard at apex
$TTL 3600
example.org.		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600
example.org.		IN	NS	b.iana-servers.net.
*.example.org.          IN      A       127.0.0.53
*.intern.example.org.   IN      A       127.0.1.52
foo.example.org.        IN      A       127.0.0.54
`
