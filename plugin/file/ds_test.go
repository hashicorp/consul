package file

import (
	"context"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
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
	zone, err := Parse(strings.NewReader(dbMiekNLDelegation), testzone, "stdin", 0)
	if err != nil {
		t.Fatalf("Expected no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range dsTestCases {
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
