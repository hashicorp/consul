// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func autopilotConfig(t retry.TestingTB, s server) *api.AutopilotConfiguration {
	conf, err := s.Client().Operator().AutopilotGetConfiguration(nil)
	require.NoError(t, err)
	return conf
}

// TestAutopilot_HealthAndState walks a three server datacenter through
// autopilot's life: defaults, health and state reports, changing the
// configuration, noticing an unhealthy server, and removing a dead one.
func TestAutopilot_HealthAndState(t *testing.T) {
	tc := newTestCluster(t, 3, "")
	leader := tc.waitForLeader(t)
	client := leader.Client()

	t.Run("defaults", func(t *testing.T) {
		conf := autopilotConfig(t, leader)
		require.True(t, conf.CleanupDeadServers)
		require.Equal(t, 200*time.Millisecond, time.Duration(*conf.LastContactThreshold))
		require.Equal(t, uint64(250), conf.MaxTrailingLogs)
		require.Equal(t, 10*time.Second, time.Duration(*conf.ServerStabilizationTime))
		require.Zero(t, conf.MinQuorum)

		out := requireCLI(t, leader, "operator", "autopilot", "get-config")
		require.Contains(t, out, "CleanupDeadServers = true")
		require.Contains(t, out, "LastContactThreshold = 200ms")
		require.Contains(t, out, "MaxTrailingLogs = 250")
		require.Contains(t, out, "ServerStabilizationTime = 10s")
	})

	t.Run("the health report lists every server as healthy", func(t *testing.T) {
		retry.RunWith(settle(), t, func(r *retry.R) {
			health, err := client.Operator().AutopilotServerHealth(nil)
			require.NoError(r, err)

			require.True(r, health.Healthy)
			require.Equal(r, 1, health.FailureTolerance)
			require.Len(r, health.Servers, 3)

			leaders := 0
			for _, sh := range health.Servers {
				require.NotNil(r, tc.byName(sh.Name), "unknown server %s", sh.Name)
				require.True(r, sh.Healthy, sh.Name)
				require.True(r, sh.Voter, sh.Name)
				require.Equal(r, "alive", sh.SerfStatus, sh.Name)
				if sh.Leader {
					leaders++
					require.Equal(r, leader.Name(), sh.Name)
				}
			}
			require.Equal(r, 1, leaders)
		})
	})

	t.Run("the state report agrees with raft", func(t *testing.T) {
		retry.RunWith(settle(), t, func(r *retry.R) {
			state, err := client.Operator().AutopilotState(nil)
			require.NoError(r, err)

			require.True(r, state.Healthy)
			require.Equal(r, 1, state.FailureTolerance)
			require.Len(r, state.Servers, 3)
			require.Len(r, state.Voters, 3)

			raftIDs := map[string]string{}
			for _, rs := range tc.raftConfig(r) {
				raftIDs[rs.Node] = rs.ID
			}
			require.Equal(r, raftIDs[leader.Name()], state.Leader)

			for id, srv := range state.Servers {
				require.Equal(r, id, srv.ID)
				require.Equal(r, raftIDs[srv.Name], id)
				require.Equal(r, "alive", srv.NodeStatus)
				require.True(r, srv.Healthy, srv.Name)
				require.False(r, srv.ReadReplica, srv.Name)
				require.Equal(r, api.AutopilotTypeVoter, srv.NodeType, srv.Name)
				if srv.Name == leader.Name() {
					require.Equal(r, api.AutopilotServerLeader, srv.Status)
				} else {
					require.Equal(r, api.AutopilotServerVoter, srv.Status, srv.Name)
				}
			}
		})
	})

	t.Run("the configuration is updated with check-and-set and the CLI", func(t *testing.T) {
		conf := autopilotConfig(t, leader)
		stale := *conf
		stale.ModifyIndex = conf.ModifyIndex - 1

		conf.CleanupDeadServers = false
		ok, err := client.Operator().AutopilotCASConfiguration(&stale, nil)
		require.NoError(t, err)
		require.False(t, ok, "a stale index must not overwrite the configuration")
		require.True(t, autopilotConfig(t, leader).CleanupDeadServers)

		ok, err = client.Operator().AutopilotCASConfiguration(conf, nil)
		require.NoError(t, err)
		require.True(t, ok)
		require.False(t, autopilotConfig(t, leader).CleanupDeadServers)

		requireCLI(t, leader, "operator", "autopilot", "set-config", "-max-trailing-logs=300", "-server-stabilization-time=15s")
		updated := autopilotConfig(t, leader)
		require.Equal(t, uint64(300), updated.MaxTrailingLogs)
		require.Equal(t, 15*time.Second, time.Duration(*updated.ServerStabilizationTime))
		require.False(t, updated.CleanupDeadServers, "options that are not passed are kept")
		require.Greater(t, updated.ModifyIndex, conf.ModifyIndex)

		out := requireCLI(t, leader, "operator", "autopilot", "get-config")
		require.Contains(t, out, "MaxTrailingLogs = 300")
		require.Contains(t, out, "ServerStabilizationTime = 15s")
	})

	var dead server
	t.Run("a failed server makes the datacenter unhealthy", func(t *testing.T) {
		for _, s := range tc.servers {
			if s.Name() != leader.Name() {
				dead = s
				break
			}
		}
		tc.halt(t, dead)

		retry.RunWith(settle(), t, func(r *retry.R) {
			health, err := client.Operator().AutopilotServerHealth(nil)
			require.NoError(r, err)
			require.False(r, health.Healthy)
			require.Zero(r, health.FailureTolerance)
			require.Len(r, health.Servers, 3, "with cleanup disabled the dead server stays")

			for _, sh := range health.Servers {
				if sh.Name == dead.Name() {
					require.False(r, sh.Healthy)
					require.Equal(r, "failed", sh.SerfStatus)
				} else {
					require.True(r, sh.Healthy, sh.Name)
				}
			}

			state, err := client.Operator().AutopilotState(nil)
			require.NoError(r, err)
			require.False(r, state.Healthy)
			for _, srv := range state.Servers {
				require.Equal(r, srv.Name != dead.Name(), srv.Healthy, srv.Name)
			}
		})
	})

	t.Run("enabling cleanup removes the dead server", func(t *testing.T) {
		requireCLI(t, leader, "operator", "autopilot", "set-config", "-cleanup-dead-servers=true")

		tc.waitForRaftPeers(t, leader, tc.anyLive(leader))
		tc.waitForMemberStatus(t, dead, "left")

		retry.RunWith(settle(), t, func(r *retry.R) {
			health, err := client.Operator().AutopilotServerHealth(nil)
			require.NoError(r, err)
			require.True(r, health.Healthy)
			require.Len(r, health.Servers, 2)
			require.Zero(r, health.FailureTolerance, "two voters cannot lose one")
		})
	})
}

// TestAutopilot_ServerPromotion verifies that a server joining a running
// datacenter is added to raft first and given a vote only after it has been
// stable and caught up.
func TestAutopilot_ServerPromotion(t *testing.T) {
	tc := newTestCluster(t, 3, "")
	tc.waitForLeader(t)

	joined := tc.addServer(t)

	tc.waitForRaftPeers(t, tc.servers...)
	tc.waitForVoters(t, tc.servers...)

	leader := tc.waitForLeader(t)
	retry.RunWith(settle(), t, func(r *retry.R) {
		health, err := leader.Client().Operator().AutopilotServerHealth(nil)
		require.NoError(r, err)
		require.True(r, health.Healthy)
		require.Equal(r, 1, health.FailureTolerance)
		require.Len(r, health.Servers, 4)

		state, err := leader.Client().Operator().AutopilotState(nil)
		require.NoError(r, err)
		require.Len(r, state.Voters, 4)
		srv := state.Servers[findID(r, tc, joined)]
		require.True(r, srv.Healthy)
		require.False(r, srv.StableSince.IsZero())
	})

	requireWrite(t, leader, "after-join", "1")
	requireStaleValue(t, joined, "after-join", "1")
}

func findID(t retry.TestingTB, tc *testCluster, s server) string {
	for _, rs := range tc.raftConfig(t) {
		if rs.Node == s.Name() {
			return rs.ID
		}
	}
	require.FailNow(t, "server not in raft", s.Name())
	return ""
}

// TestAutopilot_ConfigFromAgentConfig verifies that the autopilot stanza of the
// agent configuration seeds the cluster-wide autopilot settings.
func TestAutopilot_ConfigFromAgentConfig(t *testing.T) {
	tc := newTestCluster(t, 1, `autopilot {
  cleanup_dead_servers = false
  last_contact_threshold = "500ms"
  max_trailing_logs = 500
  server_stabilization_time = "20s"
}`)
	srv := tc.waitForLeader(t)

	conf := autopilotConfig(t, srv)
	require.False(t, conf.CleanupDeadServers)
	require.Equal(t, 500*time.Millisecond, time.Duration(*conf.LastContactThreshold))
	require.Equal(t, uint64(500), conf.MaxTrailingLogs)
	require.Equal(t, 20*time.Second, time.Duration(*conf.ServerStabilizationTime))

	out := requireCLI(t, srv, "operator", "autopilot", "get-config")
	require.Contains(t, out, "CleanupDeadServers = false")
	require.Contains(t, out, "LastContactThreshold = 500ms")
	require.Contains(t, out, "MaxTrailingLogs = 500")
	require.Contains(t, out, "ServerStabilizationTime = 20s")
}
