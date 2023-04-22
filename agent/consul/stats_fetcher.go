// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"net"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
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
	inflight     map[raft.ServerID]struct{}
	inflightLock sync.Mutex
}

// NewStatsFetcher returns a stats fetcher.
func NewStatsFetcher(logger hclog.Logger, pool *pool.ConnPool, datacenter string) *StatsFetcher {
	return &StatsFetcher{
		logger:     logger,
		pool:       pool,
		datacenter: datacenter,
		inflight:   make(map[raft.ServerID]struct{}),
	}
}

// fetch does the RPC to fetch the server stats from a single server. We don't
// cancel this when the context is canceled because we only want one in-flight
// RPC to each server, so we let it finish and then clean up the in-flight
// tracking.
func (f *StatsFetcher) fetch(server *autopilot.Server, replyCh chan *autopilot.ServerStats) {
	var args EmptyReadRequest
	var reply structs.RaftStats

	// defer some cleanup to notify everything else that the fetching is no longer occurring
	// this is easier than trying to make the conditionals line up just right.
	defer func() {
		f.inflightLock.Lock()
		delete(f.inflight, server.ID)
		f.inflightLock.Unlock()
	}()

	addr, err := net.ResolveTCPAddr("tcp", string(server.Address))
	if err != nil {
		f.logger.Warn("error resolving TCP address for server",
			"address", server.Address,
			"error", err)
		return
	}

	err = f.pool.RPC(f.datacenter, server.Name, addr, "Status.RaftStats", &args, &reply)
	if err != nil {
		f.logger.Warn("error getting server health from server",
			"server", server.Name,
			"error", err,
		)
		return
	}

	replyCh <- reply.ToAutopilotServerStats()
}

// Fetch will attempt to query all the servers in parallel.
func (f *StatsFetcher) Fetch(ctx context.Context, servers map[raft.ServerID]*autopilot.Server) map[raft.ServerID]*autopilot.ServerStats {
	type workItem struct {
		server  *autopilot.Server
		replyCh chan *autopilot.ServerStats
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
	replies := make(map[raft.ServerID]*autopilot.ServerStats)
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
