// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// recordingHealth is a Health data source that records the service names it was
// asked to watch, for asserting the intention-driven discovery reconcile.
type recordingHealth struct {
	watched map[string]struct{}
	connect map[string]bool // per service: whether the watch requested Connect endpoints
}

func (r *recordingHealth) Notify(_ context.Context, req *structs.ServiceSpecificRequest, _ string, _ chan<- UpdateEvent) error {
	if r.watched == nil {
		r.watched = make(map[string]struct{})
		r.connect = make(map[string]bool)
	}
	r.watched[req.ServiceName] = struct{}{}
	r.connect[req.ServiceName] = req.Connect
	return nil
}

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

// TestHandlerInferenceGateway_reconcileModelWatches drives the intention-driven
// discovery path: services in DiscoveredUpstreams get a health watch started,
// and services that drop out of the discovered set have their watch cancelled
// and their model dropped.
func TestHandlerInferenceGateway_reconcileModelWatches(t *testing.T) {
	health := &recordingHealth{}
	s := &handlerInferenceGateway{handlerState: handlerState{}}
	s.logger = hclog.NewNullLogger()
	s.source = &structs.QuerySource{Datacenter: "dc1"}
	s.dataSources.Health = health

	sn := structs.NewServiceName("gemini-travel", acl.DefaultEnterpriseMeta())

	snap := &ConfigSnapshot{}
	snap.InferenceGateway.WatchedModels = map[structs.ServiceName]context.CancelFunc{}
	snap.InferenceGateway.Models = map[structs.ServiceName]*InferenceGatewayModel{}

	// A discovered upstream starts a health watch.
	snap.InferenceGateway.DiscoveredUpstreams = structs.ServiceList{sn}
	require.NoError(t, s.reconcileModelWatches(context.Background(), snap))
	require.Contains(t, snap.InferenceGateway.WatchedModels, sn)
	require.Contains(t, health.watched, "gemini-travel")

	// Simulate the model being populated by its health update.
	snap.InferenceGateway.Models[sn] = &InferenceGatewayModel{Service: sn, Role: structs.AIRoleModel}

	// Dropping it from the discovered set cancels the watch and removes the model
	// (proves intention removal removes the model from routing).
	snap.InferenceGateway.DiscoveredUpstreams = structs.ServiceList{}
	require.NoError(t, s.reconcileModelWatches(context.Background(), snap))
	require.NotContains(t, snap.InferenceGateway.WatchedModels, sn)
	require.NotContains(t, snap.InferenceGateway.Models, sn)
}

// TestHandlerInferenceGateway_reconcileStoreWatch drives the rate-limit StateStore
// watch: binding a StateStore starts a Connect (mTLS) health watch on it; removing
// it cancels the watch and clears the endpoints. Unlike model discovery this is
// driven by the bound entry, not intentions, and uses Connect endpoints.
func TestHandlerInferenceGateway_reconcileStoreWatch(t *testing.T) {
	health := &recordingHealth{}
	s := &handlerInferenceGateway{handlerState: handlerState{}}
	s.logger = hclog.NewNullLogger()
	s.source = &structs.QuerySource{Datacenter: "dc1"}
	s.dataSources.Health = health

	snap := &ConfigSnapshot{}

	// No StateStore on the entry → no watch.
	snap.InferenceGateway.GatewayConfig = &structs.AIGatewayConfigEntry{}
	require.NoError(t, s.reconcileStoreWatch(context.Background(), snap))
	require.Empty(t, snap.InferenceGateway.StateStoreService.Name)
	require.NotContains(t, health.watched, "valkey")

	// Binding a StateStore starts a Connect health watch on it.
	snap.InferenceGateway.GatewayConfig = &structs.AIGatewayConfigEntry{
		StateStore: &structs.AIGatewayStateStore{Service: "valkey", LocalBindPort: 6379},
	}
	require.NoError(t, s.reconcileStoreWatch(context.Background(), snap))
	require.Equal(t, "valkey", snap.InferenceGateway.StateStoreService.Name)
	require.NotNil(t, snap.InferenceGateway.StateStoreCancel)
	require.Contains(t, health.watched, "valkey")
	require.True(t, health.connect["valkey"], "store is a Connect mesh upstream (mTLS)")

	// A repeat reconcile with the same store is a no-op (watch stays put).
	require.NoError(t, s.reconcileStoreWatch(context.Background(), snap))
	require.Equal(t, "valkey", snap.InferenceGateway.StateStoreService.Name)

	// Simulate endpoints landing via the health update.
	snap.InferenceGateway.StateStoreNodes = structs.CheckServiceNodes{
		{Service: &structs.NodeService{Service: "valkey-sidecar-proxy"}},
	}

	// Removing the StateStore cancels the watch and clears its endpoints.
	snap.InferenceGateway.GatewayConfig = &structs.AIGatewayConfigEntry{}
	require.NoError(t, s.reconcileStoreWatch(context.Background(), snap))
	require.Empty(t, snap.InferenceGateway.StateStoreService.Name)
	require.Nil(t, snap.InferenceGateway.StateStoreCancel)
	require.Empty(t, snap.InferenceGateway.StateStoreNodes)
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
