package consul

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/raft"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestStatsFetcher(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dir1, s1 := testServerDCExpect(t, "dc1", 3)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCExpect(t, "dc1", 3)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCExpect(t, "dc1", 3)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	testrpc.WaitForTestAgent(t, s2.RPC, "dc1")
	testrpc.WaitForTestAgent(t, s3.RPC, "dc1")

	members := s1.serfLAN.Members()
	if len(members) != 3 {
		t.Fatalf("bad len: %d", len(members))
	}

	for _, member := range members {
		ok, _ := metadata.IsConsulServer(member)
		if !ok {
			t.Fatalf("expected member to be a server: %#v", member)
		}
	}

	// Do a normal fetch and make sure we get three responses.
	func() {
		retry.Run(t, func(r *retry.R) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			stats := s1.statsFetcher.Fetch(ctx, s1.autopilotServers())
			if len(stats) != 3 {
				r.Fatalf("bad: %#v", stats)
			}
			for id, stat := range stats {
				switch types.NodeID(id) {
				case s1.config.NodeID, s2.config.NodeID, s3.config.NodeID:
					// OK
				default:
					r.Fatalf("bad: %s", id)
				}

				if stat == nil || stat.LastTerm == 0 {
					r.Fatalf("bad: %#v", stat)
				}
			}
		})
	}()

	// Fake an in-flight request to server 3 and make sure we don't fetch
	// from it.
	func() {
		retry.Run(t, func(r *retry.R) {
			s1.statsFetcher.inflightLock.Lock()
			s1.statsFetcher.inflight[raft.ServerID(s3.config.NodeID)] = struct{}{}
			s1.statsFetcher.inflightLock.Unlock()
			defer func() {
				s1.statsFetcher.inflightLock.Lock()
				delete(s1.statsFetcher.inflight, raft.ServerID(s3.config.NodeID))
				s1.statsFetcher.inflightLock.Unlock()
			}()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			stats := s1.statsFetcher.Fetch(ctx, s1.autopilotServers())
			if len(stats) != 2 {
				r.Fatalf("bad: %#v", stats)
			}
			for id, stat := range stats {
				switch types.NodeID(id) {
				case s1.config.NodeID, s2.config.NodeID:
					// OK
				case s3.config.NodeID:
					r.Fatalf("bad")
				default:
					r.Fatalf("bad: %s", id)
				}

				if stat == nil || stat.LastTerm == 0 {
					r.Fatalf("bad: %#v", stat)
				}
			}
		})
	}()
}
