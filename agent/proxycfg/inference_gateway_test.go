// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func TestCandidateModelServices(t *testing.T) {
	em := *acl.DefaultEnterpriseMeta()

	require.Empty(t, candidateModelServices(nil, em))

	e := &structs.AIGatewayConfigEntry{
		Routing: structs.AIGatewayRouting{
			FallbackChain: []string{"vllm-prod"},
			MatchRules: []structs.AIGatewayMatchRule{
				{Candidates: []string{"openai-gpt4", "anthropic-sonnet"}, FallbackChain: []string{"vllm-prod"}},
			},
			Scoring: &structs.AIGatewayScoring{
				WeightedSplit: []structs.AIGatewayWeightedTarget{{Cluster: "openai-gpt4", Weight: 80}},
			},
		},
	}

	got := candidateModelServices(e, em)
	require.Len(t, got, 3) // deduped across all sources
	for _, name := range []string{"openai-gpt4", "anthropic-sonnet", "vllm-prod"} {
		_, ok := got[structs.NewServiceName(name, &em)]
		require.Truef(t, ok, "expected candidate %q", name)
	}
}

func TestHandlerInferenceGateway_updateModel(t *testing.T) {
	h := &handlerInferenceGateway{}
	snap := &ConfigSnapshot{}
	snap.InferenceGateway.Models = map[structs.ServiceName]*InferenceGatewayModel{}
	sn := structs.NewServiceName("openai-gpt4", acl.DefaultEnterpriseMeta())

	modelNodes := structs.CheckServiceNodes{
		{Service: &structs.NodeService{
			Service: "openai-gpt4",
			AI:      &structs.AIConfig{Role: structs.AIRoleModel},
			Meta:    map[string]string{"provider": "openai", "tier": "premium"},
		}},
	}

	// An ai-model service is recorded with its labels.
	h.updateModel(snap, sn, modelNodes)
	got, ok := snap.InferenceGateway.Models[sn]
	require.True(t, ok)
	require.Equal(t, structs.AIRoleModel, got.Role)
	require.Equal(t, "openai", got.Labels["provider"])
	require.Equal(t, "premium", got.Labels["tier"])

	// A service with no ai-model instance is dropped.
	h.updateModel(snap, sn, structs.CheckServiceNodes{
		{Service: &structs.NodeService{Service: "openai-gpt4"}},
	})
	_, ok = snap.InferenceGateway.Models[sn]
	require.False(t, ok)
}

func TestConfigSnapshotInferenceGateway_valid(t *testing.T) {
	snap := &ConfigSnapshot{Kind: structs.ServiceKindInferenceGateway}
	require.False(t, snap.Valid())

	snap.Roots = &structs.IndexedCARoots{}
	snap.InferenceGateway.Leaf = &structs.IssuedCert{}
	snap.InferenceGateway.MeshConfigSet = true
	snap.InferenceGateway.GatewayConfigSet = true
	require.True(t, snap.Valid())

	// Leaf is required for inbound mTLS.
	snap.InferenceGateway.Leaf = nil
	require.False(t, snap.Valid())
}
