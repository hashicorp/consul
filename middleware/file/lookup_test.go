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

var dnsTestCases = []coretest.Case{
	{
		Qname: "www.miek.nl.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			coretest.CNAME("www.miek.nl.	1800	IN	CNAME	a.miek.nl."),
		},

		Extra: []dns.RR{
			coretest.A("a.miek.nl.	1800	IN	A	139.162.196.78"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeSOA,
		Answer: []dns.RR{
			coretest.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeAAAA,
		Answer: []dns.RR{
			coretest.AAAA("miek.nl.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fef1:6735"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeMX,
		Answer: []dns.RR{
			coretest.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com."),
			coretest.MX("miek.nl.	1800	IN	MX	10 aspmx2.googlemail.com."),
			coretest.MX("miek.nl.	1800	IN	MX	10 aspmx3.googlemail.com."),
			coretest.MX("miek.nl.	1800	IN	MX	5 alt1.aspmx.l.google.com."),
			coretest.MX("miek.nl.	1800	IN	MX	5 alt2.aspmx.l.google.com."),
		},
	},
	{
		Qname: "a.miek.nl.", Qtype: dns.TypeSRV,
		Ns: []dns.RR{
			coretest.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "b.miek.nl.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			coretest.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
}

const (
	testzone  = "miek.nl."
	testzone1 = "dnssex.nl."
)

func TestLookup(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbMiekNL), testzone, "stdin")
	if err != nil {
		t.Fatalf("expect no error when reading zone, got %q", err)
	}

	fm := File{Next: coretest.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range dnsTestCases {
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

func TestLookupNil(t *testing.T) {
	fm := File{Next: coretest.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: nil}, Names: []string{testzone}}}
	ctx := context.TODO()

	m := dnsTestCases[0].Msg()
	rec := middleware.NewResponseRecorder(&middleware.TestResponseWriter{})
	fm.ServeDNS(ctx, rec, m)
}

func BenchmarkLookup(b *testing.B) {
	zone, err := Parse(strings.NewReader(dbMiekNL), testzone, "stdin")
	if err != nil {
		return
	}

	fm := File{Next: coretest.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()
	rec := middleware.NewResponseRecorder(&middleware.TestResponseWriter{})

	tc := coretest.Case{
		Qname: "www.miek.nl.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			coretest.CNAME("www.miek.nl.	1800	IN	CNAME	a.miek.nl."),
		},

		Extra: []dns.RR{
			coretest.A("a.miek.nl.	1800	IN	A	139.162.196.78"),
		},
	}

	m := tc.Msg()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		fm.ServeDNS(ctx, rec, m)
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
                IN      NS      ns-ext.nlnetlabs.nl.
                IN      NS      omval.tednet.nl.
                IN      NS      ext.ns.whyscream.net.

                IN      MX      1  aspmx.l.google.com.
                IN      MX      5  alt1.aspmx.l.google.com.
                IN      MX      5  alt2.aspmx.l.google.com.
                IN      MX      10 aspmx2.googlemail.com.
                IN      MX      10 aspmx3.googlemail.com.

                IN      A       139.162.196.78
                IN      AAAA    2a01:7e00::f03c:91ff:fef1:6735

a               IN      A       139.162.196.78
                IN      AAAA    2a01:7e00::f03c:91ff:fef1:6735
www             IN      CNAME   a
archive         IN      CNAME   a`
