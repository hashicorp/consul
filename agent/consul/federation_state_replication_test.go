// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestReplication_FederationStates(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.DisableFederationStateAntiEntropy = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

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

	// Create some new federation states (weird because we're having dc1 update it for the other 50)
	var fedStateDCs []string
	for i := 0; i < 50; i++ {
		dc := fmt.Sprintf("alt-dc%d", i+1)
		ip1 := fmt.Sprintf("1.2.3.%d", i+1)
		ip2 := fmt.Sprintf("4.3.2.%d", i+1)
		arg := structs.FederationStateRequest{
			Datacenter: "dc1",
			Op:         structs.FederationStateUpsert,
			State: &structs.FederationState{
				Datacenter: dc,
				MeshGateways: []structs.CheckServiceNode{
					newTestMeshGatewayNode(
						dc, "gateway1", ip1, 443, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
					newTestMeshGatewayNode(
						dc, "gateway2", ip2, 443, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
				},
				UpdatedAt: time.Now().UTC(),
			},
		}

		out := false
		require.NoError(t, s1.RPC(context.Background(), "FederationState.Apply", &arg, &out))
		fedStateDCs = append(fedStateDCs, dc)
	}

	checkSame := func(t *retry.R) error {
		_, remote, err := s1.fsm.State().FederationStateList(nil)
		require.NoError(t, err)
		_, local, err := s2.fsm.State().FederationStateList(nil)
		require.NoError(t, err)

		require.Len(t, local, len(remote))
		for i := range remote {
			// Make lightweight copies so we can zero out the raft fields
			// without mutating the copies in memdb.
			remoteCopy := *remote[i]
			localCopy := *local[i]
			// zero out the raft data for future comparisons
			remoteCopy.RaftIndex = structs.RaftIndex{}
			localCopy.RaftIndex = structs.RaftIndex{}
			require.Equal(t, remoteCopy, localCopy)
		}
		return nil
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// Update those states
	for i := 0; i < 50; i++ {
		dc := fmt.Sprintf("alt-dc%d", i+1)
		ip1 := fmt.Sprintf("1.2.3.%d", i+1)
		ip2 := fmt.Sprintf("4.3.2.%d", i+1)
		ip3 := fmt.Sprintf("5.8.9.%d", i+1)
		arg := structs.FederationStateRequest{
			Datacenter: "dc1",
			Op:         structs.FederationStateUpsert,
			State: &structs.FederationState{
				Datacenter: dc,
				MeshGateways: []structs.CheckServiceNode{
					newTestMeshGatewayNode(
						dc, "gateway1", ip1, 8443, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
					newTestMeshGatewayNode(
						dc, "gateway2", ip2, 8443, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
					newTestMeshGatewayNode(
						dc, "gateway3", ip3, 8443, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
				},
				UpdatedAt: time.Now().UTC(),
			},
		}

		out := false
		require.NoError(t, s1.RPC(context.Background(), "FederationState.Apply", &arg, &out))
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	for _, fedStateDC := range fedStateDCs {
		arg := structs.FederationStateRequest{
			Datacenter: "dc1",
			Op:         structs.FederationStateDelete,
			State: &structs.FederationState{
				Datacenter: fedStateDC,
			},
		}

		out := false
		require.NoError(t, s1.RPC(context.Background(), "FederationState.Apply", &arg, &out))
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})
}
