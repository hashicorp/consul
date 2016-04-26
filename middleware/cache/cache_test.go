package cache

import (
	"testing"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/test"

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
				test.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com."),
				test.MX("miek.nl.	1800	IN	MX	10 aspmx2.googlemail.com."),
				test.MX("miek.nl.	1800	IN	MX	10 aspmx3.googlemail.com."),
				test.MX("miek.nl.	1800	IN	MX	5 alt1.aspmx.l.google.com."),
				test.MX("miek.nl.	1800	IN	MX	5 alt2.aspmx.l.google.com."),
			},
		},
		in: test.Case{
			Qname: "miek.nl.", Qtype: dns.TypeMX,
			Answer: []dns.RR{
				test.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com."),
				test.MX("miek.nl.	1800	IN	MX	10 aspmx2.googlemail.com."),
				test.MX("miek.nl.	1800	IN	MX	10 aspmx3.googlemail.com."),
				test.MX("miek.nl.	1800	IN	MX	5 alt1.aspmx.l.google.com."),
				test.MX("miek.nl.	1800	IN	MX	5 alt2.aspmx.l.google.com."),
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
}

func cacheMsg(m *dns.Msg, tc cacheTestCase) *dns.Msg {
	m.RecursionAvailable = tc.RecursionAvailable
	m.AuthenticatedData = tc.AuthenticatedData
	m.Authoritative = tc.Authoritative
	m.Truncated = tc.Truncated
	m.Answer = tc.in.Answer
	m.Ns = tc.in.Ns
	//	m.Extra = tc.in.Extra , not the OPT record!
	return m
}

func newTestCache() (Cache, *CachingResponseWriter) {
	c := NewCache(0, []string{"."}, nil)
	crr := NewCachingResponseWriter(nil, c.cache, time.Duration(0))
	return c, crr
}

func TestCache(t *testing.T) {
	c, crr := newTestCache()

	for _, tc := range cacheTestCases {
		m := tc.in.Msg()
		m = cacheMsg(m, tc)
		do := tc.in.Do

		mt, _ := middleware.Classify(m)
		key := cacheKey(m, mt, do)
		crr.set(m, key, mt)

		name := middleware.Name(m.Question[0].Name).Normalize()
		qtype := m.Question[0].Qtype
		i, ok := c.get(name, qtype, do)
		if !ok && !m.Truncated {
			t.Errorf("Truncated message should not have been cached")
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
