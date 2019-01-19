package file

/*
TODO(miek): move to test/ for full server testing

import (
	"context"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
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
	zone, err := Parse(strings.NewReader(dbMiekNLDNAME), testzone, "stdin", 0)
	if err != nil {
		t.Fatalf("Expect no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range dnameTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		test.SortAndCheck(t, resp, tc)
	}
}

var dnameDnssecTestCases = []test.Case{
	{
		// We have no auth section, because the test zone does not have nameservers.
		Qname: "ns.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("ns.example.org.	1800	IN	A	127.0.0.1"),
		},
	},
	{
		Qname: "dname.example.org.", Qtype: dns.TypeDNAME, Do: true,
		Answer: []dns.RR{
			test.DNAME("dname.example.org.	1800	IN	DNAME	test.example.org."),
			test.RRSIG("dname.example.org.	1800	IN	RRSIG	DNAME 5 3 1800 20170702091734 20170602091734 54282 example.org. HvXtiBM="),
		},
	},
	{
		Qname: "a.dname.example.org.", Qtype: dns.TypeA, Do: true,
		Answer: []dns.RR{
			test.CNAME("a.dname.example.org.	1800	IN	CNAME	a.test.example.org."),
			test.DNAME("dname.example.org.	1800	IN	DNAME	test.example.org."),
			test.RRSIG("dname.example.org.	1800	IN	RRSIG	DNAME 5 3 1800 20170702091734 20170602091734 54282 example.org. HvXtiBM="),
		},
	},
}

func TestLookupDNAMEDNSSEC(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbExampleDNAMESigned), testzone, "stdin", 0)
	if err != nil {
		t.Fatalf("Expect no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{"example.org.": zone}, Names: []string{"example.org."}}}
	ctx := context.TODO()

	for _, tc := range dnameDnssecTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		test.SortAndCheck(t, resp, tc)
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

const dbExampleDNAMESigned = `
; File written on Fri Jun  2 10:17:34 2017
; dnssec_signzone version 9.10.3-P4-Debian
example.org.		1800	IN SOA	a.example.org. b.example.org. (
					1282630057 ; serial
					14400      ; refresh (4 hours)
					3600       ; retry (1 hour)
					604800     ; expire (1 week)
					14400      ; minimum (4 hours)
					)
			1800	RRSIG	SOA 5 2 1800 (
					20170702091734 20170602091734 54282 example.org.
					mr5eQtFs1GubgwaCcqrpiF6Cgi822OkESPeV
					X0OJYq3JzthJjHw8TfYAJWQ2yGqhlePHir9h
					FT/uFZdYyytHq+qgIUbJ9IVCrq0gZISZdHML
					Ry1DNffMR9CpD77KocOAUABfopcvH/3UGOHn
					TFxkAr447zPaaoC68JYGxYLfZk8= )
			1800	NS	ns.example.org.
			1800	RRSIG	NS 5 2 1800 (
					20170702091734 20170602091734 54282 example.org.
					McM4UdMxkscVQkJnnEbdqwyjpPgq5a/EuOLA
					r2MvG43/cwOaWULiZoNzLi5Rjzhf+GTeVTan
					jw6EsL3gEuYI1nznwlLQ04/G0XAHjbq5VvJc
					rlscBD+dzf774yfaTjRNoeo2xTem6S7nyYPW
					Y+1f6xkrsQPLYJfZ6VZ9QqyupBw= )
			14400	NSEC	dname.example.org. NS SOA RRSIG NSEC DNSKEY
			14400	RRSIG	NSEC 5 2 14400 (
					20170702091734 20170602091734 54282 example.org.
					VT+IbjDFajM0doMKFipdX3+UXfCn3iHIxg5x
					LElp4Q/YddTbX+6tZf53+EO+G8Kye3JDLwEl
					o8VceijNeF3igZ+LiZuXCei5Qg/TJ7IAUnAO
					xd85IWwEYwyKkKd6Z2kXbAN2pdcHE8EmboQd
					wfTr9oyWhpZk1Z+pN8vdejPrG0M= )
			1800	DNSKEY	256 3 5 (
					AwEAAczLlmTk5bMXUzpBo/Jta6MWSZYy3Nfw
					gz8t/pkfSh4IlFF6vyXZhEqCeQsCBdD7ltkD
					h5qd4A+nFrYOMwsi5XIjoHMlJN15xwFS9EgS
					ZrZmuxePIEiYB5KccEf9JQMgM1t07Iu1FnrY
					02OuAqGWcO4tuyTLaK3QP4MLQOfAgKqf
					) ; ZSK; alg = RSASHA1; key id = 54282
			1800	RRSIG	DNSKEY 5 2 1800 (
					20170702091734 20170602091734 54282 example.org.
					MBgSRtZ6idJblLIHxZWpWL/1oqIwImb1mkl7
					hDFxqV6Hw19yLX06P7gcJEWiisdZBkVEfcOK
					LeMJly05vgKfrMzLgIu2Ry4bL8AMKc8NMXBG
					b1VDCEBW69P2omogj2KnORHDCZQr/BX9+wBU
					5rIMTTKlMSI5sT6ecJHHEymtiac= )
dname.example.org.	1800	IN A	127.0.0.1
			1800	RRSIG	A 5 3 1800 (
					20170702091734 20170602091734 54282 example.org.
					LPCK2nLyDdGwvmzGLkUO2atEUjoc+aEspkC3
					keZCdXZaLnAwBH7dNAjvvXzzy0WrgWeiyDb4
					+rJ2N0oaKEZicM4QQDHKhugJblKbU5G4qTey
					LSEaV3vvQnzGd0S6dCqnwfPj9czagFN7Zlf5
					DmLtdxx0aiDPCUpqT0+H/vuGPfk= )
			1800	DNAME	test.example.org.
			1800	RRSIG	DNAME 5 3 1800 (
					20170702091734 20170602091734 54282 example.org.
					HvX79T1flWJ8H9/1XZjX6gz8rP/o2jbfPXJ9
					vC7ids/ZJilSReabLru4DCqcw1IV2DM/CZdE
					tBnED/T2PJXvMut9tnYMrz+ZFPxoV6XyA3Z7
					bok3B0OuxizzAN2EXdol04VdbMHoWUzjQCzi
					0Ri12zLGRPzDepZ7FolgD+JtiBM= )
			14400	NSEC	a.dname.example.org. A DNAME RRSIG NSEC
			14400	RRSIG	NSEC 5 3 14400 (
					20170702091734 20170602091734 54282 example.org.
					U3ZPYMUBJl3wF2SazQv/kBf6ec0CH+7n0Hr9
					w6lBKkiXz7P9WQzJDVnTHEZOrbDI6UetFGyC
					6qcaADCASZ9Wxc+riyK1Hl4ox+Y/CHJ97WHy
					oS2X//vEf6qmbHQXin0WQtFdU/VCRYF40X5v
					8VfqOmrr8iKiEqXND8XNVf58mTw= )
a.dname.example.org.	1800	IN A	127.0.0.1
			1800	RRSIG	A 5 4 1800 (
					20170702091734 20170602091734 54282 example.org.
					y7RHBWZwli8SJQ4BgTmdXmYS3KGHZ7AitJCx
					zXFksMQtNoOfVEQBwnFqjAb8ezcV5u92h1gN
					i1EcuxCFiElML1XFT8dK2GnlPAga9w3oIwd5
					wzW/YHcnR0P9lF56Sl7RoIt6+jJqOdRfixS6
					TDoLoXsNbOxQ+qV3B8pU2Tam204= )
			14400	NSEC	ns.example.org. A RRSIG NSEC
			14400	RRSIG	NSEC 5 4 14400 (
					20170702091734 20170602091734 54282 example.org.
					Tmu27q3+xfONSZZtZLhejBUVtEw+83ZU1AFb
					Rsxctjry/x5r2JSxw/sgSAExxX/7tx/okZ8J
					oJqtChpsr91Kiw3eEBgINi2lCYIpMJlW4cWz
					8bYlHfR81VsKYgy/cRgrq1RRvBoJnw+nwSty
					mKPIvUtt67LAvLxJheSCEMZLCKI= )
ns.example.org.		1800	IN A	127.0.0.1
			1800	RRSIG	A 5 3 1800 (
					20170702091734 20170602091734 54282 example.org.
					mhi1SGaaAt+ndQEg5uKWKCH0HMzaqh/9dUK3
					p2wWMBrLbTZrcWyz10zRnvehicXDCasbBrer
					ZpDQnz5AgxYYBURvdPfUzx1XbNuRJRE4l5PN
					CEUTlTWcqCXnlSoPKEJE5HRf7v0xg2BrBUfM
					4mZnW2bFLwjrRQ5mm/mAmHmTROk= )
			14400	NSEC	example.org. A RRSIG NSEC
			14400	RRSIG	NSEC 5 3 14400 (
					20170702091734 20170602091734 54282 example.org.
					loHcdjX+NIWLAkUDfPSy2371wrfUvrBQTfMO
					17eO2Y9E/6PE935NF5bjQtZBRRghyxzrFJhm
					vY1Ad5ZTb+NLHvdSWbJQJog+eCc7QWp64WzR
					RXpMdvaE6ZDwalWldLjC3h8QDywDoFdndoRY
					eHOsmTvvtWWqtO6Fa5A8gmHT5HA= )
`
*/
