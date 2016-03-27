package file

import (
	"sort"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var dnsTestCases = []dnsTestCase{
	{
		Qname: "miek.nl.", Qtype: dns.TypeSOA,
		Answer: []dns.RR{
			newSOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeAAAA,
		Answer: []dns.RR{
			newAAAA("miek.nl.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fef1:6735"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeMX,
		Answer: []dns.RR{
			newMX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com."),
			newMX("miek.nl.	1800	IN	MX	10 aspmx2.googlemail.com."),
			newMX("miek.nl.	1800	IN	MX	10 aspmx3.googlemail.com."),
			newMX("miek.nl.	1800	IN	MX	5 alt1.aspmx.l.google.com."),
			newMX("miek.nl.	1800	IN	MX	5 alt2.aspmx.l.google.com."),
		},
	},
	{
		Qname: "www.miek.nl.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newCNAME("www.miek.nl.	1800	IN	CNAME	a.miek.nl."),
		},

		Extra: []dns.RR{
			newA("a.miek.nl.	1800	IN	A	139.162.196.78"),
			newAAAA("a.miek.nl.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fef1:6735"),
		},
	},
	{
		Qname: "a.miek.nl.", Qtype: dns.TypeSRV,
		Ns: []dns.RR{
			newSOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "b.miek.nl.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			newSOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
}

type rrSet []dns.RR

func (p rrSet) Len() int           { return len(p) }
func (p rrSet) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p rrSet) Less(i, j int) bool { return p[i].String() < p[j].String() }

const testzone = "miek.nl."

func TestLookup(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbMiekNL), testzone, "stdin")
	if err != nil {
		t.Fatalf("expect no error when reading zone, got %q", err)
	}

	fm := File{Next: handler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range dnsTestCases {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(tc.Qname), tc.Qtype)

		rec := middleware.NewResponseRecorder(&middleware.TestResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
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
	}
}

type dnsTestCase struct {
	Qname  string
	Qtype  uint16
	Rcode  int
	Answer []dns.RR
	Ns     []dns.RR
	Extra  []dns.RR
}

func newA(rr string) *dns.A         { r, _ := dns.NewRR(rr); return r.(*dns.A) }
func newAAAA(rr string) *dns.AAAA   { r, _ := dns.NewRR(rr); return r.(*dns.AAAA) }
func newCNAME(rr string) *dns.CNAME { r, _ := dns.NewRR(rr); return r.(*dns.CNAME) }
func newSRV(rr string) *dns.SRV     { r, _ := dns.NewRR(rr); return r.(*dns.SRV) }
func newSOA(rr string) *dns.SOA     { r, _ := dns.NewRR(rr); return r.(*dns.SOA) }
func newNS(rr string) *dns.NS       { r, _ := dns.NewRR(rr); return r.(*dns.NS) }
func newPTR(rr string) *dns.PTR     { r, _ := dns.NewRR(rr); return r.(*dns.PTR) }
func newTXT(rr string) *dns.TXT     { r, _ := dns.NewRR(rr); return r.(*dns.TXT) }
func newMX(rr string) *dns.MX       { r, _ := dns.NewRR(rr); return r.(*dns.MX) }

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

func handler() middleware.Handler {
	return middleware.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return dns.RcodeServerFailure, nil
	})
}
