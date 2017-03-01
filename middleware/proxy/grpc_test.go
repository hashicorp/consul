package proxy

import (
	"testing"
	"time"
)

func pool() []*UpstreamHost {
	return []*UpstreamHost{
		{
			Name: "localhost:10053",
		},
		{
			Name: "localhost:10054",
		},
	}
}

func TestStartupShutdown(t *testing.T) {
	upstream := &staticUpstream{
		from:        ".",
		Hosts:       pool(),
		Policy:      &Random{},
		Spray:       nil,
		FailTimeout: 10 * time.Second,
		MaxFails:    1,
	}
	g := newGrpcClient(nil, upstream)
	upstream.ex = g

	p := &Proxy{Trace: nil}
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
