package file

import (
	"sort"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var dsTestCases = []test.Case{
	{
		Qname: "a.delegated.miek.nl.", Qtype: dns.TypeDS,
		Ns: []dns.RR{
			test.NS("delegated.miek.nl.	1800	IN	NS	a.delegated.miek.nl."),
			test.NS("delegated.miek.nl.	1800	IN	NS	ns-ext.nlnetlabs.nl."),
		},
		Extra: []dns.RR{
			test.A("a.delegated.miek.nl. 1800 IN A 139.162.196.78"),
			test.AAAA("a.delegated.miek.nl. 1800 IN AAAA 2a01:7e00::f03c:91ff:fef1:6735"),
		},
	},
	{
		Qname: "_udp.delegated.miek.nl.", Qtype: dns.TypeDS,
		Ns: []dns.RR{
			test.NS("delegated.miek.nl.	1800	IN	NS	a.delegated.miek.nl."),
			test.NS("delegated.miek.nl.	1800	IN	NS	ns-ext.nlnetlabs.nl."),
		},
		Extra: []dns.RR{
			test.A("a.delegated.miek.nl. 1800 IN A 139.162.196.78"),
			test.AAAA("a.delegated.miek.nl. 1800 IN AAAA 2a01:7e00::f03c:91ff:fef1:6735"),
		},
	},
	{
		// This works *here* because we skip the server routing for DS in core/dnsserver/server.go
		Qname: "_udp.miek.nl.", Qtype: dns.TypeDS,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeDS,
		Ns: []dns.RR{
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
}

func TestLookupDS(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbMiekNLDelegation), testzone, "stdin")
	if err != nil {
		t.Fatalf("Expected no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range dsTestCases {
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
