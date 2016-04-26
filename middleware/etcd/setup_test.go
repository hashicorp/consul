// +build etcd

package etcd

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/middleware/singleflight"
	"github.com/miekg/coredns/middleware/test"
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
		Zones:      []string{"skydns.test.", "skydns_extra.test."},
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

		rec := middleware.NewResponseRecorder(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("expected no error, got: %v for %s %s\n", err, m.Question[0].Name, dns.Type(m.Question[0].Qtype))
			return
		}
		resp := rec.Msg()

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
