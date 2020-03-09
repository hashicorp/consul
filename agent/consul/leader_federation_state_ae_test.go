package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestLeader_FederationStateAntiEntropy_BlockingQuery(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.FederationStateReplicationRate = 100
		c.FederationStateReplicationBurst = 100
		c.FederationStateReplicationApplyLimit = 1000000
		c.DisableFederationStateAntiEntropy = true
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	checkSame := func(t *testing.T, expectN, expectGatewaysInDC2 int) {
		t.Helper()
		retry.Run(t, func(r *retry.R) {
			_, remote, err := s1.fsm.State().FederationStateList(nil)
			require.NoError(r, err)
			require.Len(r, remote, expectN)

			_, local, err := s2.fsm.State().FederationStateList(nil)
			require.NoError(r, err)
			require.Len(r, local, expectN)

			var fs2 *structs.FederationState
			for _, fs := range local {
				if fs.Datacenter == "dc2" {
					fs2 = fs
					break
				}
			}
			if expectGatewaysInDC2 < 0 {
				require.Nil(r, fs2)
			} else {
				require.NotNil(r, fs2)
				require.Len(r, fs2.MeshGateways, expectGatewaysInDC2)
			}
		})
	}

	gatewayCSN1 := newTestMeshGatewayNode(
		"dc2", "gateway1", "1.2.3.4", 443, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
	)
	gatewayCSN2 := newTestMeshGatewayNode(
		"dc2", "gateway2", "4.3.2.1", 443, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
	)

	// populate with some stuff
	makeFedState := func(t *testing.T, dc string, csn ...structs.CheckServiceNode) {
		t.Helper()
		arg := structs.FederationStateRequest{
			Datacenter: "dc1",
			Op:         structs.FederationStateUpsert,
			State: &structs.FederationState{
				Datacenter:   dc,
				MeshGateways: csn,
				UpdatedAt:    time.Now().UTC(),
			},
		}

		out := false
		require.NoError(t, s1.RPC("FederationState.Apply", &arg, &out))
	}

	makeGateways := func(t *testing.T, csn structs.CheckServiceNode) {
		t.Helper()
		const dc = "dc2"

		arg := structs.RegisterRequest{
			Datacenter: csn.Node.Datacenter,
			Node:       csn.Node.Node,
			Address:    csn.Node.Address,
			Service:    csn.Service,
			Checks:     csn.Checks,
		}
		var out struct{}
		require.NoError(t, s2.RPC("Catalog.Register", &arg, &out))
	}

	type result struct {
		idx        uint64
		prev, curr *structs.FederationState
		err        error
	}

	blockAgain := func(last uint64) <-chan result {
		ch := make(chan result, 1)
		go func() {
			var res result
			res.idx, res.prev, res.curr, res.err = s2.fetchFederationStateAntiEntropyDetails(&structs.QueryOptions{
				MinQueryIndex:     last,
				RequireConsistent: true,
			})
			ch <- res
		}()
		return ch
	}

	// wait for the primary to do one round of AE and replicate it
	checkSame(t, 1, -1)

	// // wait for change to be reflected as well
	// makeFedState(t, "dc2")
	// checkSame(t, 1)

	// Do the initial fetch (len0 local gateways, upstream has nil fedstate)
	res0 := <-blockAgain(0)
	require.NoError(t, res0.err)

	ch := blockAgain(res0.idx)

	// bump the local mesh gateways; should unblock query
	makeGateways(t, gatewayCSN1)

	res1 := <-ch
	require.NoError(t, res1.err)
	require.NotEqual(t, res1.idx, res0.idx)
	require.Nil(t, res1.prev)
	require.Len(t, res1.curr.MeshGateways, 1)

	checkSame(t, 1, -1) // no fed state update yet

	ch = blockAgain(res1.idx)

	// do manual AE
	makeFedState(t, "dc2", gatewayCSN1)

	res2 := <-ch
	require.NoError(t, res2.err)
	require.NotEqual(t, res2.idx, res1.idx)
	require.Len(t, res2.prev.MeshGateways, 1)
	require.Len(t, res2.curr.MeshGateways, 1)

	checkSame(t, 2, 1)

	ch = blockAgain(res2.idx)

	// add another local mesh gateway
	makeGateways(t, gatewayCSN2)

	res3 := <-ch
	require.NoError(t, res3.err)
	require.NotEqual(t, res3.idx, res2.idx)
	require.Len(t, res3.prev.MeshGateways, 1)
	require.Len(t, res3.curr.MeshGateways, 2)

	checkSame(t, 2, 1)

	ch = blockAgain(res3.idx)

	// do manual AE
	makeFedState(t, "dc2", gatewayCSN1, gatewayCSN2)

	res4 := <-ch
	require.NoError(t, res4.err)
	require.NotEqual(t, res4.idx, res3.idx)
	require.Len(t, res4.prev.MeshGateways, 2)
	require.Len(t, res4.curr.MeshGateways, 2)

	checkSame(t, 2, 2)
}

func TestLeader_FederationStateAntiEntropyPruning(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
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
