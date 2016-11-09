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

func TestLookupCNAMEChain(t *testing.T) {
	name := "example.org."
	zone, err := Parse(strings.NewReader(dbExampleCNAME), name, "stdin")
	if err != nil {
		t.Fatalf("Expected no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{name: zone}, Names: []string{name}}}
	ctx := context.TODO()

	for _, tc := range cnameTestCases {
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
dangling        IN      CNAME   foo`
