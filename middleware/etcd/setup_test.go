// +build etcd

package etcd

// etcd needs to be running on http://127.0.0.1:2379
// *and* needs connectivity to the internet for remotely resolving names.

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/etcd/singleflight"
	"github.com/miekg/coredns/middleware/proxy"
	coretest "github.com/miekg/coredns/middleware/testing"
	"github.com/miekg/dns"

	etcdc "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

var (
	etc    Etcd
	client etcdc.KeysAPI
	ctx    context.Context
)

func init() {
	ctx, _ = context.WithTimeout(context.Background(), etcdTimeout)

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

func TestLookup(t *testing.T) {
	for _, serv := range services {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	for _, tc := range dnsTestCases {
		m := tc.Msg()

		rec := middleware.NewResponseRecorder(&middleware.TestResponseWriter{})
		_, err := etc.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got %v\n", err)
			return
		}
		resp := rec.Msg()

		sort.Sort(coretest.RRSet(resp.Answer))
		sort.Sort(coretest.RRSet(resp.Ns))
		sort.Sort(coretest.RRSet(resp.Extra))

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

		if !coretest.CheckSection(t, tc, coretest.Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !coretest.CheckSection(t, tc, coretest.Ns, resp.Ns) {
			t.Logf("%v\n", resp)

		}
		if !coretest.CheckSection(t, tc, coretest.Extra, resp.Extra) {
			t.Logf("%v\n", resp)
		}
	}
}
