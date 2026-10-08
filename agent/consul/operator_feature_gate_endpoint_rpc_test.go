// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

// operator_feature_gate_endpoint_rpc_test.go covers FeatureGateGet and
// FeatureGateSet with a real server + msgpackrpc codec. Each test covers one
// documented scenario: uninitialized state, unknown feature, successful
// get/set, CAS mismatch, semantic no-op, and Raft-apply paths.

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

// waitForFeatureGateInit blocks until the leader has committed the first
// feature-gate policy/status generation (required before any set calls).
func waitForFeatureGateInit(t *testing.T, s *Server) {
	t.Helper()
	retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 50 * time.Millisecond}, t, func(r *retry.R) {
		_, policy, status, err := s.fsm.State().FeatureGatePolicyAndStatus(nil)
		require.NoError(r, err)
		require.NotNil(r, policy, "feature-gate policy not yet initialized")
		require.NotNil(r, status, "feature-gate status not yet initialized")
	})
}

// ----- FeatureGateGet -------------------------------------------------------

func TestFeatureGateGet_UnknownFeatureName(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	args := &structs.FeatureGateQueryRequest{Name: "this-does-not-exist"}
	var reply structs.FeatureGateQueryResponse
	err := msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown feature gate")
}

func TestFeatureGateGet_SingleFeature(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	featureName := featuregate.APIGatewayUpstreamRouting.String()
	args := &structs.FeatureGateQueryRequest{Name: featureName}
	var reply structs.FeatureGateQueryResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply))

	require.False(t, reply.Uninitialized)
	require.Len(t, reply.Features, 1)
	require.Equal(t, featureName, reply.Features[0].Name)
}

func TestFeatureGateGet_ListAll(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	args := &structs.FeatureGateQueryRequest{} // empty Name → list all
	var reply structs.FeatureGateQueryResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply))

	require.False(t, reply.Uninitialized)
	// There is at least one registered feature (APIGatewayUpstreamRouting).
	require.NotEmpty(t, reply.Features)
}

// ----- FeatureGateSet -------------------------------------------------------

func TestFeatureGateSet_UnknownFeature(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	args := &structs.FeatureGateSetRequest{
		Name:    "does-not-exist",
		Enabled: true,
	}
	var reply structs.FeatureGateSetResponse
	err := msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateSet", args, &reply)
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown feature gate "does-not-exist"`)
}

func TestFeatureGateSet_Successful(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	featureName := featuregate.APIGatewayUpstreamRouting.String()
	args := &structs.FeatureGateSetRequest{
		Name:    featureName,
		Enabled: true,
	}
	var reply structs.FeatureGateSetResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateSet", args, &reply))

	require.True(t, reply.Applied, "set should be applied on first write")
	require.Equal(t, featureName, reply.Feature.Name)
	require.True(t, reply.Feature.DesiredEnabled)
}

func TestFeatureGateSet_CASMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	// Read the current policy index.
	_, policy, _, err := s.fsm.State().FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.NotNil(t, policy)

	// Provide a deliberately wrong expected index.
	wrongIndex := policy.ModifyIndex + 99
	args := &structs.FeatureGateSetRequest{
		Name:                featuregate.APIGatewayUpstreamRouting.String(),
		Enabled:             true,
		ExpectedPolicyIndex: wrongIndex,
	}
	var reply structs.FeatureGateSetResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateSet", args, &reply))

	// CAS mismatch: Applied must be false, no error.
	require.False(t, reply.Applied, "CAS mismatch should return Applied=false, not an error")
}

func TestFeatureGateSet_NoOpSameSetting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	featureName := featuregate.APIGatewayUpstreamRouting.String()

	// First write: enable the feature.
	first := &structs.FeatureGateSetRequest{Name: featureName, Enabled: true}
	var firstReply structs.FeatureGateSetResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateSet", first, &firstReply))
	require.True(t, firstReply.Applied)

	// Second write: same value, same source (operator) → semantic no-op.
	second := &structs.FeatureGateSetRequest{Name: featureName, Enabled: true}
	var secondReply structs.FeatureGateSetResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateSet", second, &secondReply))

	// The endpoint short-circuits and returns Applied=true (idempotent success).
	require.True(t, secondReply.Applied, "re-applying the same setting should return Applied=true")
}

func TestFeatureGateSet_CommittedResponseReturned(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	featureName := featuregate.APIGatewayUpstreamRouting.String()
	args := &structs.FeatureGateSetRequest{Name: featureName, Enabled: true}
	var reply structs.FeatureGateSetResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateSet", args, &reply))
	require.True(t, reply.Applied)

	// The feature info in the reply must reflect the post-commit state.
	require.Equal(t, featureName, reply.Feature.Name)
	// PolicyIndex and StatusIndex must be non-zero (committed).
	require.NotZero(t, reply.Feature.PolicyIndex)
	require.NotZero(t, reply.Feature.StatusIndex)
	// Source must be operator since we wrote it.
	require.Equal(t, string(featuregate.SourceOperator), reply.Feature.Source)
}

// TestFeatureGateSet_ACLDenied verifies that a token without operator:write
// is rejected.
func TestFeatureGateSet_ACLDenied(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s, _ := testACLServerWithConfig(t, nil, false)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	args := &structs.FeatureGateSetRequest{
		Name:    featuregate.APIGatewayUpstreamRouting.String(),
		Enabled: true,
		// Token is empty → anonymous token, which has no operator:write on a
		// default-deny ACL cluster.
		WriteRequest: structs.WriteRequest{Token: ""},
	}
	var reply structs.FeatureGateSetResponse
	err := msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateSet", args, &reply)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Permission denied")
}

// TestFeatureGateGet_ACLDenied verifies that a token without operator:read
// is rejected.
func TestFeatureGateGet_ACLDenied(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s, _ := testACLServerWithConfig(t, nil, false)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")

	args := &structs.FeatureGateQueryRequest{
		DCSpecificRequest: structs.DCSpecificRequest{
			QueryOptions: structs.QueryOptions{Token: ""},
		},
	}
	var reply structs.FeatureGateQueryResponse
	err := msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Permission denied")
}

func TestFeatureGateGet_NodeIdentityAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s, codec := testACLServerWithConfig(t, nil, false)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)
	token, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(token *structs.ACLToken) {
		token.NodeIdentities = structs.ACLNodeIdentities{{NodeName: "client-1", Datacenter: "dc1"}}
	})
	require.NoError(t, err)

	args := &structs.FeatureGateQueryRequest{
		Node: "client-1",
		DCSpecificRequest: structs.DCSpecificRequest{
			Datacenter:     "dc1",
			QueryOptions:   structs.QueryOptions{Token: token.SecretID},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}
	var reply structs.FeatureGateQueryResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply))
	require.NotEmpty(t, reply.Features)

	args.Node = "another-node"
	err = msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Permission denied")
}

func TestFeatureGateGet_EffectiveNodeWriteAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	for _, roleIdentity := range []bool{false, true} {
		t.Run(fmt.Sprintf("role_identity_%t", roleIdentity), func(t *testing.T) {
			_, server, codec := testACLServerWithConfig(t, nil, false)
			testrpc.WaitForLeader(t, server.RPC, "dc1")
			waitForFeatureGateInit(t, server)
			var policyID, roleID string
			if roleIdentity {
				role, err := upsertTestCustomizedRole(codec, TestDefaultInitialManagementToken, "dc1", func(role *structs.ACLRole) {
					role.NodeIdentities = structs.ACLNodeIdentities{{NodeName: "client_one", Datacenter: "dc1"}}
				})
				require.NoError(t, err)
				roleID = role.ID
			} else {
				policy, err := upsertTestPolicyWithRules(codec, TestDefaultInitialManagementToken, "dc1", `node "client_one" { policy = "write" }`)
				require.NoError(t, err)
				policyID = policy.ID
			}
			token, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(token *structs.ACLToken) {
				if roleIdentity {
					token.Roles = []structs.ACLTokenRoleLink{{ID: roleID}}
				} else {
					token.Policies = []structs.ACLTokenPolicyLink{{ID: policyID}}
				}
			})
			require.NoError(t, err)
			args := &structs.FeatureGateQueryRequest{
				Node: "client_one",
				DCSpecificRequest: structs.DCSpecificRequest{
					Datacenter:   "dc1",
					QueryOptions: structs.QueryOptions{Token: token.SecretID},
				},
			}
			var reply structs.FeatureGateQueryResponse
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply))
			require.NotEmpty(t, reply.Features)
			args.Node = "another_node"
			err = msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply)
			require.ErrorContains(t, err, "Permission denied")
		})
	}
}

// featureGateSnapshot persists an FSM snapshot whose feature-gate status sits
// at index with every registered feature set to enabled.
func featureGateSnapshot(t *testing.T, index uint64, enabled bool) *bytes.Buffer {
	t.Helper()
	settings := map[string]structs.FeatureGateSetting{}
	resolved := map[string]structs.ResolvedFeatureGate{}
	for _, definition := range featuregate.DefaultRegistry().Definitions() {
		settings[definition.Name] = structs.FeatureGateSetting{Enabled: enabled, Source: structs.FeatureGateSourceOperator}
		resolved[definition.Name] = structs.ResolvedFeatureGate{DesiredEnabled: enabled, EffectiveEnabled: enabled, Eligible: true}
	}
	store := state.NewStateStore(nil)
	ok, err := store.FeatureGateUpdate(index, &structs.FeatureGateUpdateRequest{
		Policy: &structs.FeatureGatePolicy{Settings: settings},
		Status: &structs.FeatureGateStatus{Features: resolved},
	})
	require.NoError(t, err)
	require.True(t, ok)

	snapshotFSM := fsm.NewFromDeps(fsm.Deps{
		Logger:         hclog.NewNullLogger(),
		NewStateStore:  func() *state.Store { return store },
		StorageBackend: newTestRaftStorageBackend(t),
	})
	snap, err := snapshotFSM.Snapshot()
	require.NoError(t, err)
	sink := &bufferSnapshotSink{}
	require.NoError(t, snap.Persist(sink))
	return &sink.Buffer
}

// TestFeatureGateGet_RestoreReleasesBlockedQuery verifies a snapshot restore to
// an older status that flips a gate releases an existing blocking query with the
// restored decision, instead of leaving it blocked on the restored store's
// lower table index until the query times out.
func TestFeatureGateGet_RestoreReleasesBlockedQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	codec := rpcClient(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)
	// The reconciler would otherwise rewrite the restored status mid-test.
	s.stopFeatureGateReconciliation()

	gate := featuregate.LocalizedDNS
	restore := func(index uint64, enabled bool) {
		require.NoError(t, s.fsm.Restore(io.NopCloser(featureGateSnapshot(t, index, enabled))))
	}
	get := func(minIndex uint64) (structs.FeatureGateQueryResponse, error) {
		args := &structs.FeatureGateQueryRequest{
			Name: gate.String(),
			DCSpecificRequest: structs.DCSpecificRequest{
				Datacenter:   "dc1",
				QueryOptions: structs.QueryOptions{MinQueryIndex: minIndex, MaxQueryTime: 30 * time.Second},
			},
		}
		var reply structs.FeatureGateQueryResponse
		err := msgpackrpc.CallWithCodec(codec, "Operator.FeatureGateGet", args, &reply)
		return reply, err
	}

	restore(1000, true)
	require.Eventually(t, func() bool { return s.featureGateStore.Enabled(gate) }, 5*time.Second, 10*time.Millisecond)
	before, err := get(0)
	require.NoError(t, err)
	require.Len(t, before.Features, 1)
	require.True(t, before.Features[0].EffectiveEnabled)

	type result struct {
		reply structs.FeatureGateQueryResponse
		err   error
	}
	done := make(chan result, 1)
	go func() {
		// Like the client loop: a response that still carries the old decision is
		// followed by another poll at its index.
		index := before.Index
		for {
			reply, err := get(index)
			if err != nil || len(reply.Features) != 1 || !reply.Features[0].EffectiveEnabled {
				done <- result{reply, err}
				return
			}
			index = reply.Index
		}
	}()

	restore(10, false)

	select {
	case got := <-done:
		require.NoError(t, got.err)
		require.Len(t, got.reply.Features, 1)
		require.False(t, got.reply.Features[0].EffectiveEnabled)
		require.Greater(t, got.reply.Index, before.Index)
	case <-time.After(10 * time.Second):
		t.Fatal("the restored decision did not release the blocked query")
	}
}
