// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"net"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/sdk/testutil"
)

// newRenameTestServer builds the minimum *Server needed to drive
// lanNodeJoin / lanNodeFailed / lanNodeUpdate against a real
// ServerLookup. We deliberately avoid spinning a serf cluster or raft
// instance — the bug being reproduced lives entirely in the LAN-event
// mutators on serverLookup.
func newRenameTestServer(t *testing.T) *Server {
	t.Helper()
	logger := testutil.Logger(t)
	return &Server{
		serverLookup: NewServerLookup(),
		router:       router.NewRouter(logger, "dc1", "", nil),
		logger:       logger,
		config: &Config{
			Datacenter:      "dc1",
			BootstrapExpect: 0,
		},
	}
}

// TestServerLookup_HostnameRenameRace reproduces the bug behind production
// occurrences of structs.ErrLeaderNotTracked ("Raft leader not found in
// server lookup mapping").
//
// The underlying defect is general: ServerLookup is keyed by IP:port and
// by Raft NodeID, neither of which is unique across time. When two
// generations of a peer share a key — here, a hostname rename keeps
// IP:port and NodeID stable while changing the node-name — an unguarded
// RemoveServer call for the older generation evicts the entry that
// AddServer just installed for the live generation. Any subsequent RPC
// that hits getLeader() then returns ErrLeaderNotTracked until a later
// member event happens to resync the lookup.
//
// This test drives the rename case through the real serf-event handlers.
// TestServerLookup_RemoveServer_StaleEntryDoesNotEvictLive covers the
// general contract at the data-structure level for the rename, the
// relocation, and the address-reuse cases.
func TestServerLookup_HostnameRenameRace(t *testing.T) {
	s := newRenameTestServer(t)

	ip := net.ParseIP("10.0.0.7")
	addr := raft.ServerAddress("10.0.0.7:8300")

	// Same Raft ID and same IP:port for both generations; only the
	// hostname differs across the rename.
	const raftID = "11111111-1111-1111-1111-111111111111"
	oldM := makeTestNode(t, testMember{
		dc: "dc1", name: "host-A", id: raftID, addr: ip,
		server: true, build: "1.22.7",
	})
	newM := makeTestNode(t, testMember{
		dc: "dc1", name: "host-B", id: raftID, addr: ip,
		server: true, build: "1.22.7",
	})

	// Steady state before the rename: only the old generation is known.
	s.lanNodeJoin(serf.MemberEvent{
		Type:    serf.EventMemberJoin,
		Members: []serf.Member{*oldM},
	})
	require.NotNil(t, s.serverLookup.Server(addr),
		"precondition: old generation should be tracked after initial join")

	// The renamed node returns. Serf delivers the EventMemberJoin for
	// the new name before the EventMemberFailed for the old name on
	// this follower — a legal interleaving that is observed in
	// practice and not synchronised across followers.
	s.lanNodeJoin(serf.MemberEvent{
		Type:    serf.EventMemberJoin,
		Members: []serf.Member{*newM},
	})
	s.lanNodeFailed(serf.MemberEvent{
		Type:    serf.EventMemberFailed,
		Members: []serf.Member{*oldM},
	})

	// The currently-live peer is at addr. Any RPC that hits
	// getLeader() will look it up by this address. If lookup returns
	// nil, getLeader() returns ErrLeaderNotTracked and the caller sees
	// a 5xx until the next EventMemberUpdate happens to fire.
	got := s.serverLookup.Server(addr)
	require.NotNil(t, got,
		"live entry at %s must survive a stale EventMemberFailed for "+
			"a prior generation that shared the same address", addr)
	require.Equal(t, "host-B", got.ShortName,
		"live entry must reflect the new (live) generation")
}

// TestServerLookup_RemoveServer_StaleEntryDoesNotEvictLive locks in the
// compare-and-delete contract directly at the data-structure level. It
// covers the three known ways two metadata.Server generations can share
// one of ServerLookup's keys over time:
//
//   - rename:        same ID and Addr,   different Name.
//   - relocation:    same ID and Name,   different Addr.
//   - address reuse: different ID,       different Name (Addr key shared).
//
// In every case, removing the stale generation must not evict the live
// entry from either index it currently occupies.
func TestServerLookup_RemoveServer_StaleEntryDoesNotEvictLive(t *testing.T) {
	cases := []struct {
		name string
		// live is the entry currently in the lookup; stale is the
		// entry that a late RemoveServer call wants to remove.
		live  *metadata.Server
		stale *metadata.Server
	}{
		{
			name: "rename: same ID and Addr, different Name",
			live: &metadata.Server{
				ID: "id-1", Name: "host-B.dc1", ShortName: "host-B",
				Addr: &testAddr{addr: "10.0.0.8:8300"},
			},
			stale: &metadata.Server{
				ID: "id-1", Name: "host-A.dc1", ShortName: "host-A",
				Addr: &testAddr{addr: "10.0.0.8:8300"},
			},
		},
		{
			name: "relocation: same ID and Name, different Addr",
			live: &metadata.Server{
				ID: "id-2", Name: "host-C.dc1", ShortName: "host-C",
				Addr: &testAddr{addr: "10.0.0.10:8300"},
			},
			stale: &metadata.Server{
				ID: "id-2", Name: "host-C.dc1", ShortName: "host-C",
				Addr: &testAddr{addr: "10.0.0.9:8300"},
			},
		},
		{
			name: "address reuse: different ID and Name, shared Addr",
			live: &metadata.Server{
				ID: "id-4", Name: "host-E.dc1", ShortName: "host-E",
				Addr: &testAddr{addr: "10.0.0.11:8300"},
			},
			stale: &metadata.Server{
				ID: "id-3", Name: "host-D.dc1", ShortName: "host-D",
				Addr: &testAddr{addr: "10.0.0.11:8300"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := NewServerLookup()
			lookup.AddServer(tc.live)

			lookup.RemoveServer(tc.stale)

			// The live entry must still be reachable through every
			// key it occupied before the stale RemoveServer call.
			got := lookup.Server(raft.ServerAddress(tc.live.Addr.String()))
			require.NotNil(t, got,
				"live entry must still be reachable by its address after stale RemoveServer")
			require.Equal(t, tc.live.Name, got.Name)

			gotAddr, err := lookup.ServerAddr(raft.ServerID(tc.live.ID))
			require.NoError(t, err,
				"live entry must still be reachable by its ID after stale RemoveServer")
			require.Equal(t, raft.ServerAddress(tc.live.Addr.String()), gotAddr)
		})
	}
}

// TestServerLookup_RemoveServer_LiveEntryIsRemoved verifies that the
// compare-and-delete guard does not over-protect: a RemoveServer call
// whose argument matches the currently-tracked entry on (ID, Name, Addr)
// must still evict that entry from both indexes.
func TestServerLookup_RemoveServer_LiveEntryIsRemoved(t *testing.T) {
	live := &metadata.Server{
		ID: "id-5", Name: "host-F.dc1", ShortName: "host-F",
		Addr: &testAddr{addr: "10.0.0.12:8300"},
	}

	lookup := NewServerLookup()
	lookup.AddServer(live)

	// A fresh *metadata.Server with the same identity tuple — this is
	// what production callers (lanNodeFailed) construct from the serf
	// member event; pointer identity is not preserved.
	dying := &metadata.Server{
		ID: live.ID, Name: live.Name, ShortName: live.ShortName,
		Addr: &testAddr{addr: live.Addr.String()},
	}
	lookup.RemoveServer(dying)

	require.Nil(t, lookup.Server(raft.ServerAddress(live.Addr.String())),
		"address index must be evicted when the dying entry matches by (ID, Name, Addr)")
	_, err := lookup.ServerAddr(raft.ServerID(live.ID))
	require.Error(t, err,
		"ID index must be evicted when the dying entry matches by (ID, Name, Addr)")
}
