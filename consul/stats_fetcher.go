package consul

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/consul/agent"
	"github.com/hashicorp/consul/consul/structs"
)

// StatsFetcher makes sure there's only one in-flight request for stats at any
// given time, and allows us to have a timeout so the autopilot loop doesn't get
// blocked if there's a slow server.
type StatsFetcher struct {
	shutdownCh   <-chan struct{}
	pool         *ConnPool
	datacenter   string
	inflight     map[string]struct{}
	inflightLock sync.Mutex
}

// NewStatsFetcher returns a stats fetcher.
func NewStatsFetcher(shutdownCh <-chan struct{}, pool *ConnPool, datacenter string) *StatsFetcher {
	return &StatsFetcher{
		shutdownCh: shutdownCh,
		pool:       pool,
		datacenter: datacenter,
		inflight:   make(map[string]struct{}),
	}
}

// Fetch will attempt to get the server health for up to the timeout, and will
// also return an error immediately if there is a request still outstanding. We
// throw away results from any outstanding requests since we don't want to
// ingest stale health data.
func (f *StatsFetcher) Fetch(server *agent.Server, timeout time.Duration) (*structs.ServerStats, error) {
	// Don't allow another request if there's another one outstanding.
	f.inflightLock.Lock()
	if _, ok := f.inflight[server.ID]; ok {
		f.inflightLock.Unlock()
		return nil, fmt.Errorf("stats request already outstanding")
	}
	f.inflight[server.ID] = struct{}{}
	f.inflightLock.Unlock()

	// Make the request in a goroutine.
	errCh := make(chan error, 1)
	var reply structs.ServerStats
	go func() {
		var args struct{}
		errCh <- f.pool.RPC(f.datacenter, server.Addr, server.Version, "Status.RaftStats", &args, &reply)

		f.inflightLock.Lock()
		delete(f.inflight, server.ID)
		f.inflightLock.Unlock()
	}()

	// Wait for something to happen.
	select {
	case <-f.shutdownCh:
		return nil, fmt.Errorf("shutdown")

	case err := <-errCh:
		if err == nil {
			return &reply, nil
		} else {
			return nil, err
		}

	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout")
	}
}
