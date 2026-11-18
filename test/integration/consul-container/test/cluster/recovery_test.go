// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// TestRecovery_PeersJSON follows the documented outage recovery for a
// datacenter that lost the majority of its servers for good: stop the
// survivor, give it a peers.json naming only itself, and start it again. It
// must come back as the leader of a one server datacenter that still has all
// of the data.
func TestRecovery_PeersJSON(t *testing.T) {
	tc := newTestCluster(t, 3, keepDeadServers)
	leader := tc.waitForLeader(t)
	requireWrite(t, leader, "before-outage", "1")

	survivor := tc.anyLive(leader)
	var survivorID, survivorAddr string
	for _, rs := range tc.raftConfig(t) {
		if rs.Node == survivor.Name() {
			survivorID, survivorAddr = rs.ID, rs.Address
		}
	}
	require.NotEmpty(t, survivorID)
	requireStaleValue(t, survivor, "before-outage", "1")

	for _, s := range tc.servers {
		if s.Name() != survivor.Name() {
			tc.halt(t, s)
		}
	}
	retry.RunWith(settle(), t, func(r *retry.R) {
		addr, err := survivor.Client().Status().Leader()
		require.NoError(r, err)
		require.Empty(r, addr, "the survivor cannot elect itself without a majority")
	})

	tc.halt(t, survivor)
	survivor.WriteDataFile(t, "raft/peers.json",
		fmt.Sprintf(`[{"id":"%s","address":"%s","non_voter":false}]`, survivorID, survivorAddr))
	tc.start(t, survivor)

	leaderNow := tc.waitForLeader(t)
	require.Equal(t, survivor.Name(), leaderNow.Name())
	tc.waitForVoters(t, survivor)

	pair, _, err := survivor.Client().KV().Get("before-outage", nil)
	require.NoError(t, err, "a consistent read needs a leader, which the survivor now is")
	require.NotNil(t, pair)
	require.Equal(t, "1", string(pair.Value))

	_, err = survivor.Client().KV().Put(&api.KVPair{Key: "after-recovery", Value: []byte("2")}, nil)
	require.NoError(t, err)

	out := requireCLI(t, survivor, "operator", "raft", "list-peers")
	require.Equal(t, map[string]string{survivor.Name(): "leader"}, peerRoles(out))
}
