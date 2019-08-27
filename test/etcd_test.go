// +build etcd

package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/etcd"
	"github.com/coredns/coredns/plugin/etcd/msg"

	"github.com/miekg/dns"
	etcdcv3 "go.etcd.io/etcd/clientv3"
)

func etcdPlugin() *etcd.Etcd {
	etcdCfg := etcdcv3.Config{
		Endpoints: []string{"http://localhost:2379"},
	}
	cli, _ := etcdcv3.New(etcdCfg)
	return &etcd.Etcd{Client: cli, PathPrefix: "/skydns"}
}

func etcdPluginWithCredentials(username, password string) *etcd.Etcd {
	etcdCfg := etcdcv3.Config{
		Endpoints: []string{"http://localhost:2379"},
		Username:  username,
		Password:  password,
	}
	cli, _ := etcdcv3.New(etcdCfg)
	return &etcd.Etcd{Client: cli, PathPrefix: "/skydns"}
}

// This test starts two coredns servers (and needs etcd). Configure a stubzones in both (that will loop) and
// will then test if we detect this loop.
func TestEtcdStubLoop(t *testing.T) {
	// TODO(miek)
}

func TestEtcdStubAndProxyLookup(t *testing.T) {
	corefile := `.:0 {
    etcd skydns.local {
        stubzones
        path /skydns
        endpoint http://localhost:2379
        upstream
	fallthrough
    }
    forward . 8.8.8.8:53
}`

	ex, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer ex.Stop()

	etc := etcdPlugin()

	var ctx = context.TODO()
	for _, serv := range servicesStub { // adds example.{net,org} as stubs
		set(ctx, t, etc, serv.Key, 0, serv)
		defer delete(ctx, t, etc, serv.Key)
	}

	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	resp, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatalf("Expected to at least one RR in the answer section, got none")
	}
	if resp.Answer[0].Header().Rrtype != dns.TypeA {
		t.Errorf("Expected RR to A, got: %d", resp.Answer[0].Header().Rrtype)
	}
	if resp.Answer[0].(*dns.A).A.String() != "93.184.216.34" {
		t.Errorf("Expected 93.184.216.34, got: %s", resp.Answer[0].(*dns.A).A.String())
	}
}

var servicesStub = []*msg.Service{
	// Two tests, ask a question that should return servfail because remote it no accessible
	// and one with edns0 option added, that should return refused.
	{Host: "127.0.0.1", Port: 666, Key: "b.example.org.stub.dns.skydns.test."},
	// Actual test that goes out to the internet.
	{Host: "199.43.132.53", Key: "a.example.net.stub.dns.skydns.test."},
}

// Copied from plugin/etcd/setup_test.go
func set(ctx context.Context, t *testing.T, e *etcd.Etcd, k string, ttl time.Duration, m *msg.Service) {
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := msg.PathWithWildcard(k, e.PathPrefix)
	e.Client.KV.Put(ctx, path, string(b))
}

// Copied from plugin/etcd/setup_test.go
func delete(ctx context.Context, t *testing.T, e *etcd.Etcd, k string) {
	path, _ := msg.PathWithWildcard(k, e.PathPrefix)
	e.Client.Delete(ctx, path)
}
