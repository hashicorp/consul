// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// keepDeadServers stops autopilot from removing failed servers by itself, so
// that a test decides when a failed server leaves the raft configuration.
const keepDeadServers = `autopilot { cleanup_dead_servers = false }`

// peerRoles parses `consul operator raft list-peers`, returning node -> state.
func peerRoles(out string) map[string]string {
	roles := map[string]string{}
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if i == 0 || len(fields) < 4 {
			continue
		}
		roles[fields[0]] = fields[3]
	}
	return roles
}

// memberStates parses `consul members`, returning node -> status.
func memberStates(out string) map[string]string {
	states := map[string]string{}
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if i == 0 || len(fields) < 3 {
			continue
		}
		states[fields[0]] = fields[2]
	}
	return states
}

// TestMembership_RaftConfiguration verifies that a three server datacenter
// agrees on who its servers are and who leads, through the API and the CLI,
// and that leadership can be handed over on request.
func TestMembership_RaftConfiguration(t *testing.T) {
	tc := newTestCluster(t, 3, "")
	leader := tc.waitForLeader(t)

	t.Run("every server votes and exactly one leads", func(t *testing.T) {
		conf := tc.raftConfig(t)
		require.Len(t, conf, 3)

		leaders := 0
		var leaderAddr string
		for _, rs := range conf {
			require.True(t, rs.Voter, "%s should vote", rs.Node)
			require.NotNil(t, tc.byName(rs.Node), "unknown raft server %s", rs.Node)
			if rs.Leader {
				leaders++
				leaderAddr = rs.Address
				require.Equal(t, leader.Name(), rs.Node)
			}
		}
		require.Equal(t, 1, leaders)

		for _, s := range tc.servers {
			peers, err := s.Client().Status().Peers()
			require.NoError(t, err)
			require.Len(t, peers, 3, "status/peers on %s", s.Name())

			got, err := s.Client().Status().Leader()
			require.NoError(t, err)
			require.Equal(t, leaderAddr, got, "status/leader on %s", s.Name())
		}
	})

	t.Run("members and list-peers agree on every server", func(t *testing.T) {
		for _, s := range tc.servers {
			states := memberStates(requireCLI(t, s, "members"))
			require.Len(t, states, 3)
			for _, other := range tc.servers {
				require.Equal(t, "alive", states[other.Name()], "%s as seen by %s", other.Name(), s.Name())
			}

			roles := peerRoles(requireCLI(t, s, "operator", "raft", "list-peers"))
			require.Len(t, roles, 3)
			require.Equal(t, "leader", roles[leader.Name()], "as seen by %s", s.Name())
			for _, other := range tc.servers {
				if other.Name() != leader.Name() {
					require.Equal(t, "follower", roles[other.Name()], "%s as seen by %s", other.Name(), s.Name())
				}
			}
		}
	})

	t.Run("leadership moves to a server of choice", func(t *testing.T) {
		var target server
		var targetID string
		for _, rs := range tc.raftConfig(t) {
			if !rs.Leader {
				target, targetID = tc.byName(rs.Node), rs.ID
				break
			}
		}

		res, err := leader.Client().Operator().RaftLeaderTransfer(targetID, nil)
		require.NoError(t, err)
		require.True(t, res.Success)

		retry.RunWith(settle(), t, func(r *retry.R) {
			require.Equal(r, target.Name(), tc.currentLeader(r).Name())
		})
		requireWrite(t, target, "after-transfer", "1")
	})

	t.Run("leadership moves on request without naming a target", func(t *testing.T) {
		before := tc.waitForLeader(t)
		requireCLI(t, before, "operator", "raft", "transfer-leader")

		retry.RunWith(settle(), t, func(r *retry.R) {
			require.NotEqual(r, before.Name(), tc.currentLeader(r).Name())
		})
	})

	t.Run("a transfer to an unknown server is refused", func(t *testing.T) {
		out, code := tc.waitForLeader(t).CLI(t, "operator", "raft", "transfer-leader", "-id=00000000-0000-0000-0000-000000000000")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "Could not find address for server id")
	})
}

// TestMembership_FailureAndRecovery verifies how a datacenter reacts when
// servers fail: a new leader takes over, writes continue, a returning server
// catches up, and once a majority is gone the datacenter stops accepting
// writes until a majority is back.
func TestMembership_FailureAndRecovery(t *testing.T) {
	tc := newTestCluster(t, 3, keepDeadServers)
	leader := tc.waitForLeader(t)
	requireWrite(t, leader, "before-failure", "1")

	t.Run("a new leader is elected when the leader fails", func(t *testing.T) {
		tc.halt(t, leader)

		newLeader := tc.waitForLeader(t)
		require.NotEqual(t, leader.Name(), newLeader.Name())
		requireWrite(t, newLeader, "while-leader-down", "2")

		tc.waitForMemberStatus(t, leader, "failed")
	})

	t.Run("the failed server catches up when it returns", func(t *testing.T) {
		tc.start(t, leader)

		tc.waitForMemberStatus(t, leader, "alive")
		tc.waitForVoters(t, tc.servers...)
		requireStaleValue(t, leader, "while-leader-down", "2")
		requireStaleValue(t, leader, "before-failure", "1")
	})

	var survivor server
	t.Run("without a majority there is no leader and no writes", func(t *testing.T) {
		survivor = tc.waitForLeader(t)
		for _, s := range tc.servers {
			if s.Name() != survivor.Name() {
				tc.halt(t, s)
			}
		}

		retry.RunWith(settle(), t, func(r *retry.R) {
			leaderAddr, err := survivor.Client().Status().Leader()
			require.NoError(r, err)
			require.Empty(r, leaderAddr)
		})

		_, err := survivor.Client().KV().Put(&api.KVPair{Key: "no-quorum", Value: []byte("x")}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "No cluster leader")

		_, _, err = survivor.Client().KV().Get("before-failure", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "No cluster leader")

		// Stale reads and the agent's own endpoints keep working.
		requireStaleValue(t, survivor, "before-failure", "1")
		_, err = survivor.Client().Agent().Self()
		require.NoError(t, err)
	})

	t.Run("the datacenter recovers when a majority returns", func(t *testing.T) {
		for _, s := range tc.servers {
			if s.Name() != survivor.Name() {
				tc.start(t, s)
				break
			}
		}

		newLeader := tc.waitForLeader(t)
		requireWrite(t, newLeader, "after-recovery", "3")

		for _, s := range tc.servers {
			if tc.out[s.Name()] {
				tc.start(t, s)
			}
		}
		tc.waitForVoters(t, tc.servers...)
		for _, s := range tc.servers {
			requireStaleValue(t, s, "after-recovery", "3")
			requireStaleValue(t, s, "before-failure", "1")
		}
	})
}

// TestMembership_LeaveAndForceLeave verifies how servers are taken out of the
// datacenter: force-leave for a server that failed, a graceful leave for a
// server that is still running, and pruning a server that already left.
func TestMembership_LeaveAndForceLeave(t *testing.T) {
	tc := newTestCluster(t, 3, keepDeadServers)
	leader := tc.waitForLeader(t)

	var failed, leaver server
	for _, s := range tc.servers {
		switch {
		case s.Name() == leader.Name():
		case failed == nil:
			failed = s
		default:
			leaver = s
		}
	}

	t.Run("a failed server stays in raft until it is forced out", func(t *testing.T) {
		tc.halt(t, failed)
		tc.waitForMemberStatus(t, failed, "failed")
		require.Len(t, tc.raftConfig(t), 3)

		requireCLI(t, leader, "force-leave", failed.Name())

		tc.waitForMemberStatus(t, failed, "left")
		tc.waitForRaftPeers(t, leader, leaver)
	})

	t.Run("force-leave of an unknown node is refused", func(t *testing.T) {
		out, code := leader.CLI(t, "force-leave", "ghost")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "No node found with name 'ghost'")
	})

	t.Run("a server that left can come back", func(t *testing.T) {
		tc.start(t, failed)

		tc.waitForMemberStatus(t, failed, "alive")
		tc.waitForVoters(t, tc.servers...)
		requireWrite(t, leader, "after-return", "1")
		requireStaleValue(t, failed, "after-return", "1")
	})

	t.Run("a graceful leave removes the server from raft", func(t *testing.T) {
		tc.leave(t, leaver)

		tc.waitForMemberStatus(t, leaver, "left")
		tc.waitForRaftPeers(t, leader, failed)
		requireWrite(t, leader, "after-leave", "1")
	})

	t.Run("a server that left can be pruned from the member list", func(t *testing.T) {
		requireCLI(t, leader, "force-leave", "-prune", leaver.Name())

		retry.RunWith(settle(), t, func(r *retry.R) {
			require.NotContains(r, tc.memberStatuses(r), leaver.Name())
		})
	})
}

// TestMembership_RemovePeer verifies removing a dead server from raft by hand,
// by address and by ID, and the refusals for bad requests.
func TestMembership_RemovePeer(t *testing.T) {
	tc := newTestCluster(t, 3, keepDeadServers)
	leader := tc.waitForLeader(t)

	var follower server
	for _, s := range tc.servers {
		if s.Name() != leader.Name() {
			follower = s
			break
		}
	}
	other := tc.anyLive(leader, follower)

	peer := func(s server) (id, addr string) {
		for _, rs := range tc.raftConfig(t) {
			if rs.Node == s.Name() {
				return rs.ID, rs.Address
			}
		}
		require.FailNow(t, "server not in raft", s.Name())
		return "", ""
	}

	t.Run("bad requests are refused", func(t *testing.T) {
		out, code := leader.CLI(t, "operator", "raft", "remove-peer", "-id=00000000-0000-0000-0000-000000000000")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "was not found in the Raft configuration")

		out, code = leader.CLI(t, "operator", "raft", "remove-peer", "-address=127.0.0.1:9999")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "was not found in the Raft configuration")

		out, code = leader.CLI(t, "operator", "raft", "remove-peer")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "an address or id is required")

		require.Len(t, tc.raftConfig(t), 3, "a refused removal changes nothing")
	})

	t.Run("a dead server is removed by address", func(t *testing.T) {
		_, addr := peer(follower)
		tc.halt(t, follower)

		requireCLI(t, leader, "operator", "raft", "remove-peer", "-address="+addr)

		tc.waitForRaftPeers(t, leader, other)
		requireWrite(t, leader, "after-remove", "1")
	})

	t.Run("a returning server is added back and can be removed by ID", func(t *testing.T) {
		tc.start(t, follower)
		tc.waitForVoters(t, tc.servers...)

		id, _ := peer(follower)
		tc.halt(t, follower)

		requireCLI(t, leader, "operator", "raft", "remove-peer", "-id="+id)

		tc.waitForRaftPeers(t, leader, other)
	})
}
