package proxy

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/healthcheck"

	"google.golang.org/grpc/grpclog"
)

func pool() []*healthcheck.UpstreamHost {
	return []*healthcheck.UpstreamHost{
		{
			Name: "localhost:10053",
		},
		{
			Name: "localhost:10054",
		},
	}
}

func TestStartupShutdown(t *testing.T) {
	grpclog.SetLogger(discard{})

	upstream := &staticUpstream{
		from: ".",
		HealthCheck: healthcheck.HealthCheck{
			Hosts:       pool(),
			FailTimeout: 10 * time.Second,
			MaxFails:    1,
		},
	}
	g := newGrpcClient(nil, upstream)
	upstream.ex = g

	p := &Proxy{}
	p.Upstreams = &[]Upstream{upstream}

	err := g.OnStartup(p)
	if err != nil {
		t.Errorf("Error starting grpc client exchanger: %s", err)
		return
	}
	if len(g.clients) != len(pool()) {
		t.Errorf("Expected %d grpc clients but found %d", len(pool()), len(g.clients))
	}

	err = g.OnShutdown(p)
	if err != nil {
		t.Errorf("Error stopping grpc client exchanger: %s", err)
		return
	}
	if len(g.clients) != 0 {
		t.Errorf("Shutdown didn't remove clients, found %d", len(g.clients))
	}
	if len(g.conns) != 0 {
		t.Errorf("Shutdown didn't remove conns, found %d", len(g.conns))
	}
}

// discard is a Logger that outputs nothing.
type discard struct{}

func (d discard) Fatal(args ...interface{})                 {}
func (d discard) Fatalf(format string, args ...interface{}) {}
func (d discard) Fatalln(args ...interface{})               {}
func (d discard) Print(args ...interface{})                 {}
func (d discard) Printf(format string, args ...interface{}) {}
func (d discard) Println(args ...interface{})               {}
