// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

// operator_feature_gate_endpoint_rpc_test.go covers FeatureGateGet and
// FeatureGateSet with a real server + msgpackrpc codec. Each test covers one
// documented scenario: uninitialized state, unknown feature, successful
// get/set, CAS mismatch, semantic no-op, and Raft-apply paths.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

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
