// +build etcd

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
	ctx, _ = context.WithTimeout(ctx.Background(), etcdTimeout)

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

		if resp.Rcode != tc.Rcode {
			t.Errorf("rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
			t.Logf("%v\n", resp)
			continue
		}

		if len(resp.Answer) != len(tc.Answer) {
			t.Errorf("answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
			t.Logf("%v\n", resp)
			continue
		}
		if len(resp.Ns) != len(tc.Ns) {
			t.Errorf("authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
			t.Logf("%v\n", resp)
			continue
		}
		if len(resp.Extra) != len(tc.Extra) {
			t.Errorf("additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
			t.Logf("%v\n", resp)
			continue
		}

		if !checkSection(t, tc, Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !checkSection(t, tc, Ns, resp.Ns) {
			t.Logf("%v\n", resp)

		}
		if !checkSection(t, tc, Extra, resp.Extra) {
			t.Logf("%v\n", resp)
		}
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

func newA(rr string) *dns.A         { r, _ := dns.NewRR(rr); return r.(*dns.A) }
func newAAAA(rr string) *dns.AAAA   { r, _ := dns.NewRR(rr); return r.(*dns.AAAA) }
func newCNAME(rr string) *dns.CNAME { r, _ := dns.NewRR(rr); return r.(*dns.CNAME) }
func newSRV(rr string) *dns.SRV     { r, _ := dns.NewRR(rr); return r.(*dns.SRV) }
func newSOA(rr string) *dns.SOA     { r, _ := dns.NewRR(rr); return r.(*dns.SOA) }
func newNS(rr string) *dns.NS       { r, _ := dns.NewRR(rr); return r.(*dns.NS) }
func newPTR(rr string) *dns.PTR     { r, _ := dns.NewRR(rr); return r.(*dns.PTR) }
func newTXT(rr string) *dns.TXT     { r, _ := dns.NewRR(rr); return r.(*dns.TXT) }
func newMX(rr string) *dns.MX       { r, _ := dns.NewRR(rr); return r.(*dns.MX) }

func checkSection(t *testing.T, tc dnsTestCase, sect Section, rr []dns.RR) bool {
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
			return false
		}
		// 303 signals: don't care what the ttl is.
		if section[i].Header().Ttl != 303 && a.Header().Ttl != section[i].Header().Ttl {
			t.Errorf("Answer %d should have a Header TTL of %d, but has %d", i, section[i].Header().Ttl, a.Header().Ttl)
			return false
		}
		if a.Header().Rrtype != section[i].Header().Rrtype {
			t.Errorf("answer %d should have a header rr type of %d, but has %d", i, section[i].Header().Rrtype, a.Header().Rrtype)
			return false
		}

		switch x := a.(type) {
		case *dns.SRV:
			if x.Priority != section[i].(*dns.SRV).Priority {
				t.Errorf("answer %d should have a Priority of %d, but has %d", i, section[i].(*dns.SRV).Priority, x.Priority)
				return false
			}
			if x.Weight != section[i].(*dns.SRV).Weight {
				t.Errorf("answer %d should have a Weight of %d, but has %d", i, section[i].(*dns.SRV).Weight, x.Weight)
				return false
			}
			if x.Port != section[i].(*dns.SRV).Port {
				t.Errorf("answer %d should have a Port of %d, but has %d", i, section[i].(*dns.SRV).Port, x.Port)
				return false
			}
			if x.Target != section[i].(*dns.SRV).Target {
				t.Errorf("answer %d should have a Target of %q, but has %q", i, section[i].(*dns.SRV).Target, x.Target)
				return false
			}
		case *dns.A:
			if x.A.String() != section[i].(*dns.A).A.String() {
				t.Errorf("answer %d should have a Address of %q, but has %q", i, section[i].(*dns.A).A.String(), x.A.String())
				return false
			}
		case *dns.AAAA:
			if x.AAAA.String() != section[i].(*dns.AAAA).AAAA.String() {
				t.Errorf("answer %d should have a Address of %q, but has %q", i, section[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
				return false
			}
		case *dns.TXT:
			for j, txt := range x.Txt {
				if txt != section[i].(*dns.TXT).Txt[j] {
					t.Errorf("answer %d should have a Txt of %q, but has %q", i, section[i].(*dns.TXT).Txt[j], txt)
					return false
				}
			}
		case *dns.SOA:
			tt := section[i].(*dns.SOA)
			if x.Ns != tt.Ns {
				t.Errorf("SOA nameserver should be %q, but is %q", x.Ns, tt.Ns)
				return false
			}
		case *dns.PTR:
			tt := section[i].(*dns.PTR)
			if x.Ptr != tt.Ptr {
				t.Errorf("PTR ptr should be %q, but is %q", x.Ptr, tt.Ptr)
				return false
			}
		case *dns.CNAME:
			tt := section[i].(*dns.CNAME)
			if x.Target != tt.Target {
				t.Errorf("CNAME target should be %q, but is %q", x.Target, tt.Target)
				return false
			}
		case *dns.MX:
			tt := section[i].(*dns.MX)
			if x.Mx != tt.Mx {
				t.Errorf("MX Mx should be %q, but is %q", x.Mx, tt.Mx)
				return false
			}
			if x.Preference != tt.Preference {
				t.Errorf("MX Preference should be %q, but is %q", x.Preference, tt.Preference)
				return false
			}
		case *dns.NS:
			tt := section[i].(*dns.NS)
			if x.Ns != tt.Ns {
				t.Errorf("NS nameserver should be %q, but is %q", x.Ns, tt.Ns)
				return false
			}
		}
	}
	return true
}
