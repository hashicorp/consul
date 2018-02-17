package cache

import (
	"io/ioutil"
	"log"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/response"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

type cacheTestCase struct {
	test.Case
	in                 test.Case
	AuthenticatedData  bool
	Authoritative      bool
	RecursionAvailable bool
	Truncated          bool
	shouldCache        bool
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
		shouldCache: true,
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
		shouldCache: true,
	},
	{
		Truncated: true,
		Case: test.Case{
			Qname: "miek.nl.", Qtype: dns.TypeMX,
			Answer: []dns.RR{test.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com.")},
		},
		in:          test.Case{},
		shouldCache: false,
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
		shouldCache: true,
	},
	{
		RecursionAvailable: true, Authoritative: true,
		Case: test.Case{
			Qname: "miek.nl.", Qtype: dns.TypeMX,
			Do: true,
			Answer: []dns.RR{
				test.MX("miek.nl.	3600	IN	MX	1 aspmx.l.google.com."),
				test.MX("miek.nl.	3600	IN	MX	10 aspmx2.googlemail.com."),
				test.RRSIG("miek.nl.	3600	IN	RRSIG	MX 8 2 1800 20160521031301 20160421031301 12051 miek.nl. lAaEzB5teQLLKyDenatmyhca7blLRg9DoGNrhe3NReBZN5C5/pMQk8Jc u25hv2fW23/SLm5IC2zaDpp2Fzgm6Jf7e90/yLcwQPuE7JjS55WMF+HE LEh7Z6AEb+Iq4BWmNhUz6gPxD4d9eRMs7EAzk13o1NYi5/JhfL6IlaYy qkc="),
			},
		},
		in: test.Case{
			Qname: "miek.nl.", Qtype: dns.TypeMX,
			Do: true,
			Answer: []dns.RR{
				test.MX("miek.nl.	3600	IN	MX	1 aspmx.l.google.com."),
				test.MX("miek.nl.	3600	IN	MX	10 aspmx2.googlemail.com."),
				test.RRSIG("miek.nl.	1800	IN	RRSIG	MX 8 2 1800 20160521031301 20160421031301 12051 miek.nl. lAaEzB5teQLLKyDenatmyhca7blLRg9DoGNrhe3NReBZN5C5/pMQk8Jc u25hv2fW23/SLm5IC2zaDpp2Fzgm6Jf7e90/yLcwQPuE7JjS55WMF+HE LEh7Z6AEb+Iq4BWmNhUz6gPxD4d9eRMs7EAzk13o1NYi5/JhfL6IlaYy qkc="),
			},
		},
		shouldCache: false,
	},
	{
		RecursionAvailable: true, Authoritative: true,
		Case: test.Case{
			Qname: "example.org.", Qtype: dns.TypeMX,
			Do: true,
			Answer: []dns.RR{
				test.MX("example.org.	3600	IN	MX	1 aspmx.l.google.com."),
				test.MX("example.org.	3600	IN	MX	10 aspmx2.googlemail.com."),
				test.RRSIG("example.org.	3600	IN	RRSIG	MX 8 2 1800 20170521031301 20170421031301 12051 miek.nl. lAaEzB5teQLLKyDenatmyhca7blLRg9DoGNrhe3NReBZN5C5/pMQk8Jc u25hv2fW23/SLm5IC2zaDpp2Fzgm6Jf7e90/yLcwQPuE7JjS55WMF+HE LEh7Z6AEb+Iq4BWmNhUz6gPxD4d9eRMs7EAzk13o1NYi5/JhfL6IlaYy qkc="),
			},
		},
		in: test.Case{
			Qname: "example.org.", Qtype: dns.TypeMX,
			Do: true,
			Answer: []dns.RR{
				test.MX("example.org.	3600	IN	MX	1 aspmx.l.google.com."),
				test.MX("example.org.	3600	IN	MX	10 aspmx2.googlemail.com."),
				test.RRSIG("example.org.	1800	IN	RRSIG	MX 8 2 1800 20170521031301 20170421031301 12051 miek.nl. lAaEzB5teQLLKyDenatmyhca7blLRg9DoGNrhe3NReBZN5C5/pMQk8Jc u25hv2fW23/SLm5IC2zaDpp2Fzgm6Jf7e90/yLcwQPuE7JjS55WMF+HE LEh7Z6AEb+Iq4BWmNhUz6gPxD4d9eRMs7EAzk13o1NYi5/JhfL6IlaYy qkc="),
			},
		},
		shouldCache: true,
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
	// m.Extra = tc.in.Extra don't copy Extra, because we don't care and fake EDNS0 DO with tc.Do.
	return m
}

func newTestCache(ttl time.Duration) (*Cache, *ResponseWriter) {
	c := New()
	c.pttl = ttl
	c.nttl = ttl

	crr := &ResponseWriter{ResponseWriter: nil, Cache: c}
	return c, crr
}

func TestCache(t *testing.T) {
	now, _ := time.Parse(time.UnixDate, "Fri Apr 21 10:51:21 BST 2017")
	utc := now.UTC()

	c, crr := newTestCache(maxTTL)

	log.SetOutput(ioutil.Discard)

	for _, tc := range cacheTestCases {
		m := tc.in.Msg()
		m = cacheMsg(m, tc)
		do := tc.in.Do

		mt, _ := response.Typify(m, utc)
		k := key(m, mt, do)

		crr.set(m, k, mt, c.pttl)

		name := plugin.Name(m.Question[0].Name).Normalize()
		qtype := m.Question[0].Qtype

		i, _ := c.get(time.Now().UTC(), name, qtype, do)
		ok := i != nil

		if ok != tc.shouldCache {
			t.Errorf("cached message that should not have been cached: %s", name)
			continue
		}

		if ok {
			resp := i.toMsg(m, time.Now().UTC())

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

func TestCacheZeroTTL(t *testing.T) {
	c := New()
	c.Next = zeroTTLBackend()

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)
	ctx := context.TODO()

	c.ServeDNS(ctx, &test.ResponseWriter{}, req)
	if c.pcache.Len() != 0 {
		t.Errorf("Msg with 0 TTL should not have been cached")
	}
	if c.ncache.Len() != 0 {
		t.Errorf("Msg with 0 TTL should not have been cached")
	}
}

func BenchmarkCacheResponse(b *testing.B) {
	c := New()
	c.prefetch = 1
	c.Next = BackendHandler()

	ctx := context.TODO()

	reqs := make([]*dns.Msg, 5)
	for i, q := range []string{"example1", "example2", "a", "b", "ddd"} {
		reqs[i] = new(dns.Msg)
		reqs[i].SetQuestion(q+".example.org.", dns.TypeA)
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := reqs[i]
			c.ServeDNS(ctx, &test.ResponseWriter{}, req)
			i++
			i = i % 5
		}
	})
}

func BackendHandler() plugin.Handler {
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Response = true
		m.RecursionAvailable = true

		owner := m.Question[0].Name
		m.Answer = []dns.RR{test.A(owner + " 303 IN A 127.0.0.53")}

		w.WriteMsg(m)
		return dns.RcodeSuccess, nil
	})
}

func zeroTTLBackend() plugin.Handler {
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Response, m.RecursionAvailable = true, true

		m.Answer = []dns.RR{test.A("example.org. 0 IN A 127.0.0.53")}
		w.WriteMsg(m)
		return dns.RcodeSuccess, nil
	})
}
