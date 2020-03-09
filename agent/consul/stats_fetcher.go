package consul

import (
	"context"
	"sync"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
)

// StatsFetcher has two functions for autopilot. First, lets us fetch all the
// stats in parallel so we are taking a sample as close to the same time as
// possible, since we are comparing time-sensitive info for the health check.
// Second, it bounds the time so that one slow RPC can't hold up the health
// check loop; as a side effect of how it implements this, it also limits to
// a single in-flight RPC to any given server, so goroutines don't accumulate
// as we run the health check fairly frequently.
type StatsFetcher struct {
	logger       hclog.Logger
	pool         *pool.ConnPool
	datacenter   string
	inflight     map[string]struct{}
	inflightLock sync.Mutex
}

// NewStatsFetcher returns a stats fetcher.
func NewStatsFetcher(logger hclog.Logger, pool *pool.ConnPool, datacenter string) *StatsFetcher {
	return &StatsFetcher{
		logger:     logger,
		pool:       pool,
		datacenter: datacenter,
		inflight:   make(map[string]struct{}),
	}
}

// fetch does the RPC to fetch the server stats from a single server. We don't
// cancel this when the context is canceled because we only want one in-flight
// RPC to each server, so we let it finish and then clean up the in-flight
// tracking.
func (f *StatsFetcher) fetch(server *metadata.Server, replyCh chan *autopilot.ServerStats) {
	var args struct{}
	var reply autopilot.ServerStats
	err := f.pool.RPC(f.datacenter, server.ShortName, server.Addr, server.Version, "Status.RaftStats", server.UseTLS, &args, &reply)
	if err != nil {
		f.logger.Warn("error getting server health from server",
			"server", server.Name,
			"error", err,
		)
	} else {
		replyCh <- &reply
	}

	f.inflightLock.Lock()
	delete(f.inflight, server.ID)
	f.inflightLock.Unlock()
}

// Fetch will attempt to query all the servers in parallel.
func (f *StatsFetcher) Fetch(ctx context.Context, members []serf.Member) map[string]*autopilot.ServerStats {
	type workItem struct {
		server  *metadata.Server
		replyCh chan *autopilot.ServerStats
	}
	var servers []*metadata.Server
	for _, s := range members {
		if ok, parts := metadata.IsConsulServer(s); ok {
			servers = append(servers, parts)
		}
	}

	// Skip any servers that have inflight requests.
	var work []*workItem
	f.inflightLock.Lock()
	for _, server := range servers {
		if _, ok := f.inflight[server.ID]; ok {
			f.logger.Warn("error getting server health from server",
				"server", server.Name,
				"error", "last request still outstanding",
			)
		} else {
			workItem := &workItem{
				server:  server,
				replyCh: make(chan *autopilot.ServerStats, 1),
			}
			work = append(work, workItem)
			f.inflight[server.ID] = struct{}{}
			go f.fetch(workItem.server, workItem.replyCh)
		}
	}
	f.inflightLock.Unlock()

	// Now wait for the results to come in, or for the context to be
	// canceled.
	replies := make(map[string]*autopilot.ServerStats)
	for _, workItem := range work {
		// Drain the reply first if there is one.
		select {
		case reply := <-workItem.replyCh:
			replies[workItem.server.ID] = reply
			continue
		default:
		}

		select {
		case reply := <-workItem.replyCh:
			replies[workItem.server.ID] = reply

		case <-ctx.Done():
			f.logger.Warn("error getting server health from server",
				"server", workItem.server.Name,
				"error", ctx.Err(),
			)

			f.inflightLock.Lock()
			delete(f.inflight, workItem.server.ID)
			f.inflightLock.Unlock()
		}
	}
	return replies
}
