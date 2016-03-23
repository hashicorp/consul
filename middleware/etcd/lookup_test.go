// +build net

package etcd

// etcd needs to be running on http://127.0.0.1:2379
// *and* needs connectivity to the internet for remotely resolving
// names.

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/etcd/singleflight"
	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/dns"

	etcdc "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

var (
	etc    Etcd
	client etcdc.KeysAPI
	ctx    context.Context
)

type Section int

const (
	Answer Section = iota
	Ns
	Extra
)

func init() {
	ctx = context.TODO()

	etcdCfg := etcdc.Config{
		Endpoints: []string{"http://localhost:2379"},
	}
	cli, _ := etcdc.New(etcdCfg)
	etc = Etcd{
		Proxy:      proxy.New([]string{"8.8.8.8:53"}),
		PathPrefix: "skydns",
		Ctx:        context.Background(),
		Inflight:   &singleflight.Group{},
		Zones:      []string{"skydns.test."},
		Client:     etcdc.NewKeysAPI(cli),
	}
}

func set(t *testing.T, e Etcd, k string, ttl time.Duration, m *msg.Service) {
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := e.PathWithWildcard(k)
	e.Client.Set(ctx, path, string(b), &etcdc.SetOptions{TTL: ttl})
}

func delete(t *testing.T, e Etcd, k string) {
	path, _ := e.PathWithWildcard(k)
	e.Client.Delete(ctx, path, &etcdc.DeleteOptions{Recursive: false})
}

type rrSet []dns.RR

func (p rrSet) Len() int           { return len(p) }
func (p rrSet) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p rrSet) Less(i, j int) bool { return p[i].String() < p[j].String() }

func TestLookup(t *testing.T) {
	for _, serv := range services {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCases {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(tc.Qname), tc.Qtype)

		rec := middleware.NewResponseRecorder(&middleware.TestResponseWriter{})
		_, err := etc.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}
		resp := rec.Reply()

		sort.Sort(rrSet(resp.Answer))
		sort.Sort(rrSet(resp.Ns))
		sort.Sort(rrSet(resp.Extra))

		t.Logf("%v\n", resp)

		if resp.Rcode != tc.Rcode {
			t.Errorf("rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
			continue
		}

		if len(resp.Answer) != len(tc.Answer) {
			t.Errorf("answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
			continue
		}
		if len(resp.Ns) != len(tc.Ns) {
			t.Errorf("authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
			continue
		}
		if len(resp.Extra) != len(tc.Extra) {
			t.Errorf("additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
			continue
		}

		checkSection(t, tc, Answer, resp.Answer)
		checkSection(t, tc, Ns, resp.Ns)
		checkSection(t, tc, Extra, resp.Extra)
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

// Note the key is encoded as DNS name, while in "reality" it is a etcd path.
var services = []*msg.Service{
	{Host: "server1", Port: 8080, Key: "a.server1.dev.region1.skydns.test."},
	{Host: "10.0.0.1", Port: 8080, Key: "a.server1.prod.region1.skydns.test."},
	{Host: "10.0.0.2", Port: 8080, Key: "b.server1.prod.region1.skydns.test."},
	{Host: "::1", Port: 8080, Key: "b.server6.prod.region1.skydns.test."},

	// CNAME dedup Test
	{Host: "www.miek.nl", Key: "a.miek.nl.skydns.test."},
	{Host: "www.miek.nl", Key: "b.miek.nl.skydns.test."},

	// Unresolvable internal name
	{Host: "unresolvable.skydns.test", Key: "cname.prod.region1.skydns.test."},
}

var dnsTestCases = []dnsTestCase{
	// SRV Test
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("a.server1.dev.region1.skydns.test. 300 SRV 10 100 8080 server1.")},
	},
	// NXDOMAIN Test
	{
		Qname: "doesnotexist.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0"),
		},
	},
	// A Test
	{
		Qname: "a.server1.prod.region1.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{newA("a.server1.prod.region1.skydns.test. 300 A 10.0.0.1")},
	},
	// SRV Test where target is IP address
	{
		Qname: "a.server1.prod.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("a.server1.prod.region1.skydns.test. 300 SRV 10 100 8080 a.server1.prod.region1.skydns.test.")},
		Extra:  []dns.RR{newA("a.server1.prod.region1.skydns.test. 300 A 10.0.0.1")},
	},
	// AAAA Test
	{
		Qname: "b.server6.prod.region1.skydns.test.", Qtype: dns.TypeAAAA,
		Answer: []dns.RR{newAAAA("b.server6.prod.region1.skydns.test. 300 AAAA ::1")},
	},
	// Multiple A Record Test
	{
		Qname: "server1.prod.region1.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newA("server1.prod.region1.skydns.test. 300 A 10.0.0.1"),
			newA("server1.prod.region1.skydns.test. 300 A 10.0.0.2"),
		},
	},
	// Multi SRV with the same target, should be dedupped.
	{
		Qname: "*.miek.nl.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("*.miek.nl.skydns.test. 300 IN SRV 10 100 0 www.miek.nl."),
		},
		// TODO(miek): bit stupid to rely on my home DNS setup for this...
		Extra: []dns.RR{
			// 303 ttl: don't care for the ttl on these RRs.
			newA("a.miek.nl. 303 IN A 139.162.196.78"),
			newAAAA("a.miek.nl. 303 IN AAAA 2a01:7e00::f03c:91ff:fef1:6735"),
			newCNAME("www.miek.nl. 303 IN CNAME a.miek.nl."),
		},
	},
	// CNAME (unresolvable internal name)
	{
		Qname: "cname.prod.region1.skydns.test.", Qtype: dns.TypeA,
		Ns: []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
	/*
		// CNAME (resolvable external name)
		{
			Qname: "external1.cname.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				newA("a.miek.nl. 60 IN A 139.162.196.78"),
				newCNAME("external1.cname.skydns.test. 60 IN CNAME www.miek.nl."),
				newCNAME("www.miek.nl. 60 IN CNAME a.miek.nl."),
			},
		},
		// CNAME (unresolvable external name)
		{
			Qname: "external2.cname.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{},
			Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
		},
		// CNAME loop detection
		{
			Qname: "3.cname.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{},
			Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
		},
		// Priority Test
		{
			Qname: "region6.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{newSRV("region6.skydns.test. 300 SRV 333 100 80 server4.")},
		},
		// Subdomain Test
		{
			Qname: "region1.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{
				newSRV("region1.skydns.test. 300 SRV 10 33 0 104.server1.dev.region1.skydns.test."),
				newSRV("region1.skydns.test. 300 SRV 10 33 80 server2"),
				newSRV("region1.skydns.test. 300 SRV 10 33 8080 server1.")},
			Extra: []dns.RR{newA("104.server1.dev.region1.skydns.test. 300 A 10.0.0.1")},
		},
		// Subdomain Weight Test
		{
			Qname: "region5.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{
				newSRV("region5.skydns.test. 300 SRV 10 22 0 server2."),
				newSRV("region5.skydns.test. 300 SRV 10 36 0 server1."),
				newSRV("region5.skydns.test. 300 SRV 10 41 0 server3."),
				newSRV("region5.skydns.test. 300 SRV 30 100 0 server4.")},
		},
		// Wildcard Test
		{
			Qname: "*.region1.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{
				newSRV("*.region1.skydns.test. 300 SRV 10 33 0 104.server1.dev.region1.skydns.test."),
				newSRV("*.region1.skydns.test. 300 SRV 10 33 80 server2"),
				newSRV("*.region1.skydns.test. 300 SRV 10 33 8080 server1.")},
			Extra: []dns.RR{newA("104.server1.dev.region1.skydns.test. 300 A 10.0.0.1")},
		},
		// Wildcard Test
		{
			Qname: "prod.*.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{
				newSRV("prod.*.skydns.test. 300 IN SRV 10 50 0 105.server3.prod.region2.skydns.test."),
				newSRV("prod.*.skydns.test. 300 IN SRV 10 50 80 server2.")},
			Extra: []dns.RR{newAAAA("105.server3.prod.region2.skydns.test. 300 IN AAAA 2001::8:8:8:8")},
		},
		// Wildcard Test
		{
			Qname: "prod.any.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{
				newSRV("prod.any.skydns.test. 300 IN SRV 10 50 0 105.server3.prod.region2.skydns.test."),
				newSRV("prod.any.skydns.test. 300 IN SRV 10 50 80 server2.")},
			Extra: []dns.RR{newAAAA("105.server3.prod.region2.skydns.test. 300 IN AAAA 2001::8:8:8:8")},
		},
		// NODATA Test
		{
			Qname: "104.server1.dev.region1.skydns.test.", Qtype: dns.TypeTXT,
			Ns: []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
		},
		// NODATA Test 2
		{
			Qname: "100.server1.dev.region1.skydns.test.", Qtype: dns.TypeA,
			Rcode: dns.RcodeSuccess,
			Ns:    []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
		},
		{
			// One has group, the other has not...  Include the non-group always.
			Qname: "dom2.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				newA("dom2.skydns.test. IN A 127.0.0.1"),
				newA("dom2.skydns.test. IN A 127.0.0.2"),
			},
		},
		{
			// The groups differ.
			Qname: "dom1.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				newA("dom1.skydns.test. IN A 127.0.0.1"),
			},
		},
	*/
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

func checkSection(t *testing.T, tc dnsTestCase, sect Section, rr []dns.RR) {
	section := []dns.RR{}
	switch sect {
	case 0:
		section = tc.Answer
	case 1:
		section = tc.Ns
	case 2:
		section = tc.Extra
	}

	for i, a := range rr {
		if a.Header().Name != section[i].Header().Name {
			t.Errorf("answer %d should have a Header Name of %q, but has %q", i, section[i].Header().Name, a.Header().Name)
			continue
		}
		// 303 signals: don't care what the ttl is.
		if section[i].Header().Ttl != 303 && a.Header().Ttl != section[i].Header().Ttl {
			t.Errorf("Answer %d should have a Header TTL of %d, but has %d", i, section[i].Header().Ttl, a.Header().Ttl)
			continue
		}
		if a.Header().Rrtype != section[i].Header().Rrtype {
			t.Errorf("answer %d should have a header rr type of %d, but has %d", i, section[i].Header().Rrtype, a.Header().Rrtype)
			continue
		}

		switch x := a.(type) {
		case *dns.SRV:
			if x.Priority != section[i].(*dns.SRV).Priority {
				t.Errorf("answer %d should have a Priority of %d, but has %d", i, section[i].(*dns.SRV).Priority, x.Priority)
			}
			if x.Weight != section[i].(*dns.SRV).Weight {
				t.Errorf("answer %d should have a Weight of %d, but has %d", i, section[i].(*dns.SRV).Weight, x.Weight)
			}
			if x.Port != section[i].(*dns.SRV).Port {
				t.Errorf("answer %d should have a Port of %d, but has %d", i, section[i].(*dns.SRV).Port, x.Port)
			}
			if x.Target != section[i].(*dns.SRV).Target {
				t.Errorf("answer %d should have a Target of %q, but has %q", i, section[i].(*dns.SRV).Target, x.Target)
			}
		case *dns.A:
			if x.A.String() != section[i].(*dns.A).A.String() {
				t.Errorf("answer %d should have a Address of %q, but has %q", i, section[i].(*dns.A).A.String(), x.A.String())
			}
		case *dns.AAAA:
			if x.AAAA.String() != section[i].(*dns.AAAA).AAAA.String() {
				t.Errorf("answer %d should have a Address of %q, but has %q", i, section[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
			}
		case *dns.TXT:
			for j, txt := range x.Txt {
				if txt != section[i].(*dns.TXT).Txt[j] {
					t.Errorf("answer %d should have a Txt of %q, but has %q", i, section[i].(*dns.TXT).Txt[j], txt)
				}
			}
		case *dns.SOA:
			tt := section[i].(*dns.SOA)
			if x.Ns != tt.Ns {
				t.Errorf("SOA nameserver should be %q, but is %q", x.Ns, tt.Ns)
			}
		case *dns.PTR:
			tt := section[i].(*dns.PTR)
			if x.Ptr != tt.Ptr {
				t.Errorf("PTR ptr should be %q, but is %q", x.Ptr, tt.Ptr)
			}
		case *dns.CNAME:
			tt := section[i].(*dns.CNAME)
			if x.Target != tt.Target {
				t.Errorf("CNAME target should be %q, but is %q", x.Target, tt.Target)
			}
		case *dns.MX:
			tt := section[i].(*dns.MX)
			if x.Mx != tt.Mx {
				t.Errorf("MX Mx should be %q, but is %q", x.Mx, tt.Mx)
			}
			if x.Preference != tt.Preference {
				t.Errorf("MX Preference should be %q, but is %q", x.Preference, tt.Preference)
			}
		case *dns.NS:
			tt := section[i].(*dns.NS)
			if x.Ns != tt.Ns {
				t.Errorf("NS nameserver should be %q, but is %q", x.Ns, tt.Ns)
			}
		}
	}
}
