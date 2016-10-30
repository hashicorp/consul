// +build etcd

package etcd

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mholt/caddy"
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

func newEtcdMiddleware() *Etcd {
	ctxt, _ = context.WithTimeout(context.Background(), etcdTimeout)

	endpoints := []string{"http://localhost:2379"}
	client, _ := newEtcdClient(endpoints, "", "", "")

	return &Etcd{
		Proxy:      proxy.New([]string{"8.8.8.8:53"}),
		PathPrefix: "skydns",
		Ctx:        context.Background(),
		Inflight:   &singleflight.Group{},
		Zones:      []string{"skydns.test.", "skydns_extra.test.", "in-addr.arpa."},
		Client:     client,
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

func TestSetupEtcd(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedPath       string
		expectedEndpoint   string
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		// positive
		{
			`etcd`, false, "skydns", "http://localhost:2379", "",
		},
		{
			`etcd skydns.local {
	endpoint localhost:300
}
`, false, "skydns", "localhost:300", "",
		},
		// negative
		{
			`etcd {
	endpoints localhost:300
}
`, true, "", "", "unknown property 'endpoints'",
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		etcd, _ /*stubzones*/, err := etcdParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
				continue
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
				continue
			}
		}

		if !test.shouldErr && etcd.PathPrefix != test.expectedPath {
			t.Errorf("Etcd not correctly set for input %s. Expected: %s, actual: %s", test.input, test.expectedPath, etcd.PathPrefix)
		}
		if !test.shouldErr && etcd.endpoints[0] != test.expectedEndpoint { // only checks the first
			t.Errorf("Etcd not correctly set for input %s. Expected: '%s', actual: '%s'", test.input, test.expectedEndpoint, etcd.endpoints[0])
		}
	}
}

var ctxt context.Context
