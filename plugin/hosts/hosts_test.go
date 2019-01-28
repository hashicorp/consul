package hosts

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func (h *Hostsfile) parseReader(r io.Reader) { h.hmap = h.parse(r, h.inline) }

func TestLookupA(t *testing.T) {
	h := Hosts{Next: test.ErrorHandler(), Hostsfile: &Hostsfile{Origins: []string{"."}}}
	h.parseReader(strings.NewReader(hostsExample))

	ctx := context.TODO()

	for _, tc := range hostsTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := h.ServeDNS(ctx, rec, m)
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

var hostsTestCases = []test.Case{
	{
		Qname: "example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("example.org. 3600	IN	A 10.0.0.1"),
		},
	},
	{
		Qname: "example.com.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("example.com. 3600	IN	A 10.0.0.2"),
		},
	},
	{
		Qname: "localhost.", Qtype: dns.TypeAAAA,
		Answer: []dns.RR{
			test.AAAA("localhost. 3600	IN	AAAA ::1"),
		},
	},
	{
		Qname: "1.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Answer: []dns.RR{
			test.PTR("1.0.0.10.in-addr.arpa. 3600 PTR example.org."),
		},
	},
	{
		Qname: "2.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Answer: []dns.RR{
			test.PTR("2.0.0.10.in-addr.arpa. 3600 PTR example.com."),
		},
	},
	{
		Qname: "1.0.0.127.in-addr.arpa.", Qtype: dns.TypePTR,
		Answer: []dns.RR{
			test.PTR("1.0.0.127.in-addr.arpa. 3600 PTR localhost."),
			test.PTR("1.0.0.127.in-addr.arpa. 3600 PTR localhost.domain."),
		},
	},
	{
		Qname: "example.org.", Qtype: dns.TypeAAAA,
		Answer: []dns.RR{},
	},
	{
		Qname: "example.org.", Qtype: dns.TypeMX,
		Answer: []dns.RR{},
	},
}

const hostsExample = `
127.0.0.1 localhost localhost.domain
::1 localhost localhost.domain
10.0.0.1 example.org
::FFFF:10.0.0.2 example.com
`
