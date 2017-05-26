package file

import (
	"sort"
	"strings"
	"testing"

	"github.com/coredns/coredns/middleware/pkg/dnsrecorder"
	"github.com/coredns/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// RFC 6672, Section 2.2. Assuming QTYPE != DNAME.
var dnameSubstitutionTestCases = []struct {
	qname    string
	owner    string
	target   string
	expected string
}{
	{"com.", "example.com.", "example.net.", ""},
	{"example.com.", "example.com.", "example.net.", ""},
	{"a.example.com.", "example.com.", "example.net.", "a.example.net."},
	{"a.b.example.com.", "example.com.", "example.net.", "a.b.example.net."},
	{"ab.example.com.", "b.example.com.", "example.net.", ""},
	{"foo.example.com.", "example.com.", "example.net.", "foo.example.net."},
	{"a.x.example.com.", "x.example.com.", "example.net.", "a.example.net."},
	{"a.example.com.", "example.com.", "y.example.net.", "a.y.example.net."},
	{"cyc.example.com.", "example.com.", "example.com.", "cyc.example.com."},
	{"cyc.example.com.", "example.com.", "c.example.com.", "cyc.c.example.com."},
	{"shortloop.x.x.", "x.", ".", "shortloop.x."},
	{"shortloop.x.", "x.", ".", "shortloop."},
}

func TestDNAMESubstitution(t *testing.T) {
	for i, tc := range dnameSubstitutionTestCases {
		result := substituteDNAME(tc.qname, tc.owner, tc.target)
		if result != tc.expected {
			if result == "" {
				result = "<no match>"
			}

			t.Errorf("Case %d: Expected %s -> %s, got %v", i, tc.qname, tc.expected, result)
			return
		}
	}
}

var dnameTestCases = []test.Case{
	{
		Qname: "dname.miek.nl.", Qtype: dns.TypeDNAME,
		Answer: []dns.RR{
			test.DNAME("dname.miek.nl.	1800	IN	DNAME	test.miek.nl."),
		},
		Ns: miekAuth,
	},
	{
		Qname: "dname.miek.nl.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("dname.miek.nl.	1800	IN	A	127.0.0.1"),
		},
		Ns: miekAuth,
	},
	{
		Qname: "dname.miek.nl.", Qtype: dns.TypeMX,
		Answer: []dns.RR{},
		Ns: []dns.RR{
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "a.dname.miek.nl.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.CNAME("a.dname.miek.nl.	1800	IN	CNAME	a.test.miek.nl."),
			test.A("a.test.miek.nl.	1800	IN	A	139.162.196.78"),
			test.DNAME("dname.miek.nl.	1800	IN	DNAME	test.miek.nl."),
		},
		Ns: miekAuth,
	},
	{
		Qname: "www.dname.miek.nl.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("a.test.miek.nl.	1800	IN	A	139.162.196.78"),
			test.DNAME("dname.miek.nl.	1800	IN	DNAME	test.miek.nl."),
			test.CNAME("www.dname.miek.nl.	1800	IN	CNAME	www.test.miek.nl."),
			test.CNAME("www.test.miek.nl.	1800	IN	CNAME	a.test.miek.nl."),
		},
		Ns: miekAuth,
	},
}

func TestLookupDNAME(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbMiekNLDNAME), testzone, "stdin")
	if err != nil {
		t.Fatalf("Expect no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range dnameTestCases {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v\n", err)
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

const dbMiekNLDNAME = `
$TTL    30M
$ORIGIN miek.nl.
@       IN      SOA     linode.atoom.net. miek.miek.nl. (
                             1282630057 ; Serial
                             4H         ; Refresh
                             1H         ; Retry
                             7D         ; Expire
                             4H )       ; Negative Cache TTL
                IN      NS      linode.atoom.net.
                IN      NS      ns-ext.nlnetlabs.nl.
                IN      NS      omval.tednet.nl.
                IN      NS      ext.ns.whyscream.net.

test            IN      MX      1  aspmx.l.google.com.
                IN      MX      5  alt1.aspmx.l.google.com.
                IN      MX      5  alt2.aspmx.l.google.com.
                IN      MX      10 aspmx2.googlemail.com.
                IN      MX      10 aspmx3.googlemail.com.
a.test          IN      A       139.162.196.78
                IN      AAAA    2a01:7e00::f03c:91ff:fef1:6735
www.test        IN      CNAME   a.test

dname           IN      DNAME   test
dname           IN      A       127.0.0.1
a.dname         IN      A       127.0.0.1
`
