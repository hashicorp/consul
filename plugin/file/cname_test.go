package file

import (
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestLookupCNAMEChain(t *testing.T) {
	name := "example.org."
	zone, err := Parse(strings.NewReader(dbExampleCNAME), name, "stdin", 0)
	if err != nil {
		t.Fatalf("Expected no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{name: zone}, Names: []string{name}}}
	ctx := context.TODO()

	for _, tc := range cnameTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v\n", err)
			return
		}

		resp := rec.Msg
		test.SortAndCheck(t, resp, tc)
	}
}

var cnameTestCases = []test.Case{
	{
		Qname: "a.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("a.example.org. 1800	IN	A 127.0.0.1"),
		},
	},
	{
		Qname: "www3.example.org.", Qtype: dns.TypeCNAME,
		Answer: []dns.RR{
			test.CNAME("www3.example.org. 1800	IN	CNAME www2.example.org."),
		},
	},
	{
		Qname: "dangling.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.CNAME("dangling.example.org. 1800	IN	CNAME foo.example.org."),
		},
	},
	{
		Qname: "www3.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("a.example.org. 1800	IN	A 127.0.0.1"),
			test.CNAME("www.example.org. 1800	IN	CNAME a.example.org."),
			test.CNAME("www1.example.org. 1800	IN	CNAME www.example.org."),
			test.CNAME("www2.example.org. 1800	IN	CNAME www1.example.org."),
			test.CNAME("www3.example.org. 1800	IN	CNAME www2.example.org."),
		},
	},
}

func TestLookupCNAMEExternal(t *testing.T) {
	name := "example.org."
	zone, err := Parse(strings.NewReader(dbExampleCNAME), name, "stdin", 0)
	if err != nil {
		t.Fatalf("Expected no error when reading zone, got %q", err)
	}
	zone.Proxy = proxy.NewLookup([]string{"8.8.8.8:53"}) // TODO(miek): point to local instance

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{name: zone}, Names: []string{name}}}
	ctx := context.TODO()

	for _, tc := range exernalTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v\n", err)
			return
		}

		resp := rec.Msg
		test.SortAndCheck(t, resp, tc)
	}
}

var exernalTestCases = []test.Case{
	{
		Qname: "external.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.CNAME("external.example.org. 1800	CNAME	www.example.net."),
			// magic 303 TTL that says: don't check TTL.
			test.A("www.example.net.	303	IN	A	93.184.216.34"),
		},
	},
}

const dbExampleCNAME = `
$TTL    30M
$ORIGIN example.org.
@       IN      SOA     linode.atoom.net. miek.miek.nl. (
                             1282630057 ; Serial
                             4H         ; Refresh
                             1H         ; Retry
                             7D         ; Expire
                             4H )       ; Negative Cache TTL

a               IN      A       127.0.0.1
www3            IN      CNAME   www2
www2            IN      CNAME   www1
www1            IN      CNAME   www
www             IN      CNAME   a
dangling        IN      CNAME   foo
external        IN      CNAME   www.example.net.`
