// +build etcd

package etcd

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/pkg/singleflight"
	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/middleware/test"

	etcdc "github.com/coreos/etcd/client"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func init() {
	ctxt, _ = context.WithTimeout(context.Background(), etcdTimeout)
}

//	etc    *Etcd
func newEtcdMiddleware() *Etcd {
	ctxt, _ = context.WithTimeout(context.Background(), etcdTimeout)

	etcdCfg := etcdc.Config{
		Endpoints: []string{"http://localhost:2379"},
	}
	cli, _ := etcdc.New(etcdCfg)
	return &Etcd{
		Proxy:      proxy.New([]string{"8.8.8.8:53"}),
		PathPrefix: "skydns",
		Ctx:        context.Background(),
		Inflight:   &singleflight.Group{},
		Zones:      []string{"skydns.test.", "skydns_extra.test.", "in-addr.arpa."},
		Client:     etcdc.NewKeysAPI(cli),
	}
}

func set(t *testing.T, e *Etcd, k string, ttl time.Duration, m *msg.Service) {
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := msg.PathWithWildcard(k, e.PathPrefix)
	e.Client.Set(ctxt, path, string(b), &etcdc.SetOptions{TTL: ttl})
}

func delete(t *testing.T, e *Etcd, k string) {
	path, _ := msg.PathWithWildcard(k, e.PathPrefix)
	e.Client.Delete(ctxt, path, &etcdc.DeleteOptions{Recursive: false})
}

func TestLookup(t *testing.T) {
	etc := newEtcdMiddleware()
	for _, serv := range services {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}

	for _, tc := range dnsTestCases {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil {
			t.Errorf("expected no error, got: %v for %s %s\n", err, m.Question[0].Name, dns.Type(m.Question[0].Qtype))
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

var ctxt context.Context
