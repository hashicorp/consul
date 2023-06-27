// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"
	uuid "github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestFederationState_Apply_Upsert(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.DisableFederationStateAntiEntropy = true
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	joinWAN(t, s2, s1)
	// wait for cross-dc queries to work
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// update the primary with data from a secondary by way of request forwarding
	fedState := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}
	federationStateUpsert(t, codec2, "", fedState)

	// the previous RPC should not return until the primary has been updated but will return
	// before the secondary has the data.
	state := s1.fsm.State()
	_, fedState2, err := state.FederationStateGet(nil, "dc1")
	require.NoError(t, err)
	require.NotNil(t, fedState2)
	zeroFedStateIndexes(t, fedState2)
	require.Equal(t, fedState, fedState2)

	retry.Run(t, func(r *retry.R) {
		// wait for replication to happen
		state := s2.fsm.State()
		_, fedState2Again, err := state.FederationStateGet(nil, "dc1")
		require.NoError(r, err)
		require.NotNil(r, fedState2Again)

		// this test is not testing that the federation states that are
		// replicated are correct as that's done elsewhere.
	})

	updated := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway3", "9.9.9.9", 7777, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}
	federationStateUpsert(t, codec2, "", updated)

	state = s1.fsm.State()
	_, fedState2, err = state.FederationStateGet(nil, "dc1")
	require.NoError(t, err)
	require.NotNil(t, fedState2)
	zeroFedStateIndexes(t, fedState2)
	require.Equal(t, updated, fedState2)
}

func TestFederationState_Apply_Upsert_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL tokens
	opReadToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `operator = "read"`)
	require.NoError(t, err)

	opWriteToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `operator = "write"`)
	require.NoError(t, err)

	expected := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}

	{ // This should fail since we don't have write perms.
		args := structs.FederationStateRequest{
			Datacenter:   "dc1",
			Op:           structs.FederationStateUpsert,
			State:        expected,
			WriteRequest: structs.WriteRequest{Token: opReadToken.SecretID},
		}
		out := false
		err := msgpackrpc.CallWithCodec(codec, "FederationState.Apply", &args, &out)
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	}

	{ // This should work.
		args := structs.FederationStateRequest{
			Datacenter:   "dc1",
			Op:           structs.FederationStateUpsert,
			State:        expected,
			WriteRequest: structs.WriteRequest{Token: opWriteToken.SecretID},
		}
		out := false
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "FederationState.Apply", &args, &out))
	}

	// the previous RPC should not return until the primary has been updated but will return
	// before the secondary has the data.
	state := s1.fsm.State()
	_, got, err := state.FederationStateGet(nil, "dc1")
	require.NoError(t, err)
	require.NotNil(t, got)
	zeroFedStateIndexes(t, got)
	require.Equal(t, expected, got)
}

func TestFederationState_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	expected := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}
	federationStateUpsert(t, codec, "", expected)

	args := structs.FederationStateQuery{
		Datacenter:       "dc1",
		TargetDatacenter: "dc1",
	}
	var out structs.FederationStateResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "FederationState.Get", &args, &out))

	zeroFedStateIndexes(t, out.State)
	require.Equal(t, expected, out.State)
}

func TestFederationState_Get_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL tokens
	nadaToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
	service "foo" { policy = "write" }`)
	require.NoError(t, err)

	opReadToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
	operator = "read"`)
	require.NoError(t, err)

	// create some dummy stuff to look up
	expected := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}
	federationStateUpsert(t, codec, "root", expected)

	{ // This should fail
		args := structs.FederationStateQuery{
			Datacenter:       "dc1",
			TargetDatacenter: "dc1",
			QueryOptions:     structs.QueryOptions{Token: nadaToken.SecretID},
		}
		var out structs.FederationStateResponse
		err := msgpackrpc.CallWithCodec(codec, "FederationState.Get", &args, &out)
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	}

	{ // This should work
		args := structs.FederationStateQuery{
			Datacenter:       "dc1",
			TargetDatacenter: "dc1",
			QueryOptions:     structs.QueryOptions{Token: opReadToken.SecretID},
		}
		var out structs.FederationStateResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "FederationState.Get", &args, &out))

		zeroFedStateIndexes(t, out.State)
		require.Equal(t, expected, out.State)
	}
}

func TestFederationState_List(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.DisableFederationStateAntiEntropy = true
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	joinWAN(t, s2, s1)
	// wait for cross-dc queries to work
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// create some dummy data
	expected := structs.IndexedFederationStates{
		States: []*structs.FederationState{
			{
				Datacenter: "dc1",
				MeshGateways: []structs.CheckServiceNode{
					newTestMeshGatewayNode(
						"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
					newTestMeshGatewayNode(
						"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
				},
				UpdatedAt: time.Now().UTC(),
			},
			{
				Datacenter: "dc2",
				MeshGateways: []structs.CheckServiceNode{
					newTestMeshGatewayNode(
						"dc2", "gateway1", "5.6.7.8", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
					newTestMeshGatewayNode(
						"dc2", "gateway2", "8.7.6.5", 1111, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	federationStateUpsert(t, codec, "", expected.States[0])
	federationStateUpsert(t, codec, "", expected.States[1])

	// we'll also test the other list endpoint at the same time since the setup is nearly the same
	expectedMeshGateways := structs.DatacenterIndexedCheckServiceNodes{
		DatacenterNodes: map[string]structs.CheckServiceNodes{
			"dc1": expected.States[0].MeshGateways,
			"dc2": expected.States[1].MeshGateways,
		},
	}

	t.Run("List", func(t *testing.T) {
		args := structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var out structs.IndexedFederationStates
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "FederationState.List", &args, &out))

		for i := range out.States {
			zeroFedStateIndexes(t, out.States[i])
		}

		require.Equal(t, expected.States, out.States)
	})
	t.Run("ListMeshGateways", func(t *testing.T) {
		args := structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var out structs.DatacenterIndexedCheckServiceNodes
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "FederationState.ListMeshGateways", &args, &out))

		require.Equal(t, expectedMeshGateways.DatacenterNodes, out.DatacenterNodes)
	})
}

func TestFederationState_List_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.Datacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	joinWAN(t, s2, s1)
	// wait for cross-dc queries to work
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Create the ACL tokens
	nadaToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", ` `)
	require.NoError(t, err)

	opReadToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
	operator = "read"`)
	require.NoError(t, err)

	svcReadToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
	service_prefix "" { policy = "read" }`)
	require.NoError(t, err)

	nodeReadToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
	node_prefix "" { policy = "read" }`)
	require.NoError(t, err)

	svcAndNodeReadToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
	service_prefix "" { policy = "read" }
	node_prefix "" { policy = "read" }`)
	require.NoError(t, err)

	// create some dummy data
	expected := structs.IndexedFederationStates{
		States: []*structs.FederationState{
			{
				Datacenter: "dc1",
				MeshGateways: []structs.CheckServiceNode{
					newTestMeshGatewayNode(
						"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
					newTestMeshGatewayNode(
						"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
				},
				UpdatedAt: time.Now().UTC(),
			},
			{
				Datacenter: "dc2",
				MeshGateways: []structs.CheckServiceNode{
					newTestMeshGatewayNode(
						"dc2", "gateway1", "5.6.7.8", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
					newTestMeshGatewayNode(
						"dc2", "gateway2", "8.7.6.5", 1111, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
					),
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	federationStateUpsert(t, codec, "root", expected.States[0])
	federationStateUpsert(t, codec, "root", expected.States[1])

	// we'll also test the other list endpoint at the same time since the setup is nearly the same
	expectedMeshGateways := structs.DatacenterIndexedCheckServiceNodes{
		DatacenterNodes: map[string]structs.CheckServiceNodes{
			"dc1": expected.States[0].MeshGateways,
			"dc2": expected.States[1].MeshGateways,
		},
	}

	type tcase struct {
		token string

		listDenied       bool
		listEmpty        bool
		gwListEmpty      bool
		gwFilteredByACLs bool
	}

	cases := map[string]tcase{
		"no token": {
			token:       "",
			listDenied:  true,
			gwListEmpty: true,
		},
		"no perms": {
			token:            nadaToken.SecretID,
			listDenied:       true,
			gwListEmpty:      true,
			gwFilteredByACLs: true,
		},
		"service:read": {
			token:            svcReadToken.SecretID,
			listDenied:       true,
			gwListEmpty:      true,
			gwFilteredByACLs: true,
		},
		"node:read": {
			token:            nodeReadToken.SecretID,
			listDenied:       true,
			gwListEmpty:      true,
			gwFilteredByACLs: true,
		},
		"service:read and node:read": {
			token:      svcAndNodeReadToken.SecretID,
			listDenied: true,
		},
		"operator:read": {
			token:            opReadToken.SecretID,
			gwListEmpty:      true,
			gwFilteredByACLs: true,
		},
		"initial management token": {
			token: "root",
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Run("List", func(t *testing.T) {
				args := structs.DCSpecificRequest{
					Datacenter:   "dc1",
					QueryOptions: structs.QueryOptions{Token: tc.token},
				}
				var out structs.IndexedFederationStates
				err := msgpackrpc.CallWithCodec(codec, "FederationState.List", &args, &out)

				if tc.listDenied {
					if !acl.IsErrPermissionDenied(err) {
						t.Fatalf("err: %v", err)
					}
				} else if tc.listEmpty {
					require.NoError(t, err)
					require.Len(t, out.States, 0)
				} else {
					require.NoError(t, err)

					for i := range out.States {
						zeroFedStateIndexes(t, out.States[i])
					}

					require.Equal(t, expected.States, out.States)
				}
			})

			t.Run("ListMeshGateways", func(t *testing.T) {
				args := structs.DCSpecificRequest{
					Datacenter:   "dc1",
					QueryOptions: structs.QueryOptions{Token: tc.token},
				}
				var out structs.DatacenterIndexedCheckServiceNodes
				err := msgpackrpc.CallWithCodec(codec, "FederationState.ListMeshGateways", &args, &out)

				if tc.gwListEmpty {
					require.NoError(t, err)
					require.Len(t, out.DatacenterNodes, 0)
				} else {
					require.NoError(t, err)
					require.Equal(t, expectedMeshGateways.DatacenterNodes, out.DatacenterNodes)
				}
				require.Equal(t,
					tc.gwFilteredByACLs,
					out.QueryMeta.ResultsFilteredByACLs,
					"ResultsFilteredByACLs should be %v", tc.gwFilteredByACLs,
				)
			})
		})
	}
}

func TestFederationState_Apply_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	joinWAN(t, s2, s1)
	// wait for cross-dc queries to work
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Create a dummy federation state in the state store to look up.
	fedState := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}
	federationStateUpsert(t, codec, "", fedState)

	// Verify it's there
	state := s1.fsm.State()
	_, existing, err := state.FederationStateGet(nil, "dc1")
	require.NoError(t, err)
	zeroFedStateIndexes(t, existing)
	require.Equal(t, fedState, existing)

	retry.Run(t, func(r *retry.R) {
		// wait for it to be replicated into the secondary dc
		state := s2.fsm.State()
		_, fedState2Again, err := state.FederationStateGet(nil, "dc1")
		require.NoError(r, err)
		require.NotNil(r, fedState2Again)
	})

	// send the delete request to dc2 - it should get forwarded to dc1.
	args := structs.FederationStateRequest{
		Op:    structs.FederationStateDelete,
		State: fedState,
	}
	out := false
	require.NoError(t, msgpackrpc.CallWithCodec(codec2, "FederationState.Apply", &args, &out))

	// Verify the entry was deleted.
	_, existing, err = s1.fsm.State().FederationStateGet(nil, "dc1")
	require.NoError(t, err)
	require.Nil(t, existing)

	// verify it gets deleted from the secondary too
	retry.Run(t, func(r *retry.R) {
		_, existing, err := s2.fsm.State().FederationStateGet(nil, "dc1")
		require.NoError(r, err)
		require.Nil(r, existing)
	})
}

func TestFederationState_Apply_Delete_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DisableFederationStateAntiEntropy = true
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL tokens
	opReadToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
operator = "read"`)
	require.NoError(t, err)

	opWriteToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
operator = "write"`)
	require.NoError(t, err)

	// Create a dummy federation state in the state store to look up.
	fedState := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}
	federationStateUpsert(t, codec, "root", fedState)

	{ // This should not work
		args := structs.FederationStateRequest{
			Op:           structs.FederationStateDelete,
			State:        fedState,
			WriteRequest: structs.WriteRequest{Token: opReadToken.SecretID},
		}
		out := false
		err := msgpackrpc.CallWithCodec(codec, "FederationState.Apply", &args, &out)
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	}

	{ // This should work
		args := structs.FederationStateRequest{
			Op:           structs.FederationStateDelete,
			State:        fedState,
			WriteRequest: structs.WriteRequest{Token: opWriteToken.SecretID},
		}
		out := false
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "FederationState.Apply", &args, &out))
	}

	// Verify the entry was deleted.
	state := s1.fsm.State()
	_, existing, err := state.FederationStateGet(nil, "dc1")
	require.NoError(t, err)
	require.Nil(t, existing)
}

func newTestGatewayList(
	ip1 string, port1 int, meta1 map[string]string,
	ip2 string, port2 int, meta2 map[string]string,
) structs.CheckServiceNodes {
	return []structs.CheckServiceNode{
		{
			Node: &structs.Node{
				ID:         "664bac9f-4de7-4f1b-ad35-0e5365e8f329",
				Node:       "gateway1",
				Datacenter: "dc1",
				Address:    ip1,
			},
			Service: &structs.NodeService{
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Port:    port1,
				Meta:    meta1,
			},
			Checks: []*structs.HealthCheck{
				{
					Name:      "web connectivity",
					Status:    api.HealthPassing,
					ServiceID: "mesh-gateway",
				},
			},
		},
		{
			Node: &structs.Node{
				ID:         "3fb9a696-8209-4eee-a1f7-48600deb9716",
				Node:       "gateway2",
				Datacenter: "dc1",
				Address:    ip2,
			},
			Service: &structs.NodeService{
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Port:    port2,
				Meta:    meta2,
			},
			Checks: []*structs.HealthCheck{
				{
					Name:      "web connectivity",
					Status:    api.HealthPassing,
					ServiceID: "mesh-gateway",
				},
			},
		},
	}
}

func newTestMeshGatewayNode(
	datacenter, node string,
	ip string,
	port int,
	meta map[string]string,
	healthStatus string,
) structs.CheckServiceNode {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}

	return structs.CheckServiceNode{
		Node: &structs.Node{
			ID:         types.NodeID(id),
			Node:       node,
			Datacenter: datacenter,
			Address:    ip,
		},
		Service: &structs.NodeService{
			ID:      "mesh-gateway",
			Service: "mesh-gateway",
			Kind:    structs.ServiceKindMeshGateway,
			Port:    port,
			Meta:    meta,
		},
		Checks: []*structs.HealthCheck{
			{
				Name:      "web connectivity",
				Status:    healthStatus,
				ServiceID: "mesh-gateway",
			},
		},
	}
}

func federationStateUpsert(t *testing.T, codec rpc.ClientCodec, token string, fedState *structs.FederationState) {
	dup := *fedState
	fedState2 := &dup

	args := structs.FederationStateRequest{
		Op:           structs.FederationStateUpsert,
		State:        fedState2,
		WriteRequest: structs.WriteRequest{Token: token},
	}
	out := false
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "FederationState.Apply", &args, &out))
	require.True(t, out)
}

func zeroFedStateIndexes(t *testing.T, fedState *structs.FederationState) {
	require.NotNil(t, fedState)
	require.True(t, fedState.PrimaryModifyIndex > 0, "this should be set")
	fedState.PrimaryModifyIndex = 0          // zero out so the equality works
	fedState.RaftIndex = structs.RaftIndex{} // zero these out so the equality works
}
