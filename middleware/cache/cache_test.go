package cache

import (
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/response"
	"github.com/miekg/coredns/middleware/test"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

type cacheTestCase struct {
	test.Case
	in                 test.Case
	AuthenticatedData  bool
	Authoritative      bool
	RecursionAvailable bool
	Truncated          bool
}

var cacheTestCases = []cacheTestCase{
	{
		RecursionAvailable: true, AuthenticatedData: true, Authoritative: true,
		Case: test.Case{
			Qname: "miek.nl.", Qtype: dns.TypeMX,
			Answer: []dns.RR{
				test.MX("miek.nl.	3600	IN	MX	1 aspmx.l.google.com."),
				test.MX("miek.nl.	3600	IN	MX	10 aspmx2.googlemail.com."),
			},
		},
		in: test.Case{
			Qname: "miek.nl.", Qtype: dns.TypeMX,
			Answer: []dns.RR{
				test.MX("miek.nl.	3601	IN	MX	1 aspmx.l.google.com."),
				test.MX("miek.nl.	3601	IN	MX	10 aspmx2.googlemail.com."),
			},
		},
	},
	{
		RecursionAvailable: true, AuthenticatedData: true, Authoritative: true,
		Case: test.Case{
			Qname: "mIEK.nL.", Qtype: dns.TypeMX,
			Answer: []dns.RR{
				test.MX("mIEK.nL.	3600	IN	MX	1 aspmx.l.google.com."),
				test.MX("mIEK.nL.	3600	IN	MX	10 aspmx2.googlemail.com."),
			},
		},
		in: test.Case{
			Qname: "mIEK.nL.", Qtype: dns.TypeMX,
			Answer: []dns.RR{
				test.MX("mIEK.nL.	3601	IN	MX	1 aspmx.l.google.com."),
				test.MX("mIEK.nL.	3601	IN	MX	10 aspmx2.googlemail.com."),
			},
		},
	},
	{
		Truncated: true,
		Case: test.Case{
			Qname: "miek.nl.", Qtype: dns.TypeMX,
			Answer: []dns.RR{test.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com.")},
		},
		in: test.Case{},
	},
	{
		RecursionAvailable: true, Authoritative: true,
		Case: test.Case{
			Rcode: dns.RcodeNameError,
			Qname: "example.org.", Qtype: dns.TypeA,
			Ns: []dns.RR{
				test.SOA("example.org. 3600 IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2016082540 7200 3600 1209600 3600"),
			},
		},
		in: test.Case{
			Rcode: dns.RcodeNameError,
			Qname: "example.org.", Qtype: dns.TypeA,
			Ns: []dns.RR{
				test.SOA("example.org. 3600 IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2016082540 7200 3600 1209600 3600"),
			},
		},
	},
}

func cacheMsg(m *dns.Msg, tc cacheTestCase) *dns.Msg {
	m.RecursionAvailable = tc.RecursionAvailable
	m.AuthenticatedData = tc.AuthenticatedData
	m.Authoritative = tc.Authoritative
	m.Rcode = tc.Rcode
	m.Truncated = tc.Truncated
	m.Answer = tc.in.Answer
	m.Ns = tc.in.Ns
	//	m.Extra = tc.in.Extra , not the OPT record!
	return m
}

func newTestCache(ttl time.Duration) (*Cache, *ResponseWriter) {
	c := &Cache{Zones: []string{"."}, pcap: defaultCap, ncap: defaultCap, pttl: ttl, nttl: ttl}
	c.pcache, _ = lru.New(c.pcap)
	c.ncache, _ = lru.New(c.ncap)

	crr := &ResponseWriter{nil, c}
	return c, crr
}

func TestCache(t *testing.T) {
	c, crr := newTestCache(maxTTL)

	log.SetOutput(ioutil.Discard)

	for _, tc := range cacheTestCases {
		m := tc.in.Msg()
		m = cacheMsg(m, tc)
		do := tc.in.Do

		mt, _ := response.Typify(m)
		k := key(m, mt, do)
		crr.set(m, k, mt, c.pttl)

		name := middleware.Name(m.Question[0].Name).Normalize()
		qtype := m.Question[0].Qtype
		i, ok, _ := c.get(name, qtype, do)
		if ok && m.Truncated {
			t.Errorf("Truncated message should not have been cached")
			continue
		}

		if ok {
			resp := i.toMsg(m)

			if !test.Header(t, tc.Case, resp) {
				t.Logf("%v\n", resp)
				continue
			}

			if !test.Section(t, tc.Case, test.Answer, resp.Answer) {
				t.Logf("%v\n", resp)
			}
			if !test.Section(t, tc.Case, test.Ns, resp.Ns) {
				t.Logf("%v\n", resp)

			}
			if !test.Section(t, tc.Case, test.Extra, resp.Extra) {
				t.Logf("%v\n", resp)
			}
		}
	}
}
