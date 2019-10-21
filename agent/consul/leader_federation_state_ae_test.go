package consul

import (
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestLeader_FederationStateAntiEntropyPruning(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.DisableFederationStateDatacenterNameValidation = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.DisableFederationStateDatacenterNameValidation = true
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	checkSame := func(r *retry.R) error {
		_, remote, err := s1.fsm.State().FederationStateList(nil)
		require.NoError(r, err)
		_, local, err := s2.fsm.State().FederationStateList(nil)
		require.NoError(r, err)

		require.Len(r, remote, 2)
		require.Len(r, local, 2)
		for i, _ := range remote {
			// zero out the raft data for future comparisons
			remote[i].RaftIndex = structs.RaftIndex{}
			local[i].RaftIndex = structs.RaftIndex{}
			require.Equal(r, remote[i], local[i])
		}
		return nil
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// Now leave and shutdown dc2.
	require.NoError(t, s2.Leave())
	require.NoError(t, s2.Shutdown())

	// Wait until we know the router is updated.
	retry.Run(t, func(r *retry.R) {
		dcs := s1.router.GetDatacenters()
		require.Len(r, dcs, 1)
		require.Equal(r, "dc1", dcs[0])
	})

	// Since the background routine is going to run every hour, it likely is
	// not going to run during this test, so it's safe to directly invoke the
	// core method.
	require.NoError(t, s1.pruneStaleFederationStates())

	// Wait for dc2 to drop out.
	retry.Run(t, func(r *retry.R) {
		_, mine, err := s1.fsm.State().FederationStateList(nil)
		require.NoError(r, err)

		require.Len(r, mine, 1)
		require.Equal(r, "dc1", mine[0].Datacenter)
	})
}

func TestLeader_FederationStateAntiEntropyPruning_ACLDeny(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.DisableFederationStateDatacenterNameValidation = true
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.DisableFederationStateDatacenterNameValidation = true
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	// Create the ACL token.
	opWriteToken, err := upsertTestTokenWithPolicyRules(client, "root", "dc1", `operator = "write"`)
	require.NoError(t, err)

	require.True(t, s1.tokens.UpdateReplicationToken(opWriteToken.SecretID, token.TokenSourceAPI))
	require.True(t, s2.tokens.UpdateReplicationToken(opWriteToken.SecretID, token.TokenSourceAPI))

	checkSame := func(r *retry.R) error {
		_, remote, err := s1.fsm.State().FederationStateList(nil)
		require.NoError(r, err)
		_, local, err := s2.fsm.State().FederationStateList(nil)
		require.NoError(r, err)

		require.Len(r, remote, 2)
		require.Len(r, local, 2)
		for i, _ := range remote {
			// zero out the raft data for future comparisons
			remote[i].RaftIndex = structs.RaftIndex{}
			local[i].RaftIndex = structs.RaftIndex{}
			require.Equal(r, remote[i], local[i])
		}
		return nil
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// Now leave and shutdown dc2.
	require.NoError(t, s2.Leave())
	require.NoError(t, s2.Shutdown())

	// Wait until we know the router is updated.
	retry.Run(t, func(r *retry.R) {
		dcs := s1.router.GetDatacenters()
		require.Len(r, dcs, 1)
		require.Equal(r, "dc1", dcs[0])
	})

	// Since the background routine is going to run every hour, it likely is
	// not going to run during this test, so it's safe to directly invoke the
	// core method.
	require.NoError(t, s1.pruneStaleFederationStates())

	// Wait for dc2 to drop out.
	retry.Run(t, func(r *retry.R) {
		_, mine, err := s1.fsm.State().FederationStateList(nil)
		require.NoError(r, err)

		require.Len(r, mine, 1)
		require.Equal(r, "dc1", mine[0].Datacenter)
	})
}
