// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/go-testing-interface"
)

// TestConfigSnapshotInferenceGateway returns a fully-populated inference gateway
// snapshot: its own leaf (inbound mesh mTLS), the bound ai-gateway routing
// policy (ext_proc UDS + a match rule + fallback), and one discovered ai-model
// upstream with catalog labels.
func TestConfigSnapshotInferenceGateway(t testing.T, nsFn func(ns *structs.NodeService), extraUpdates []UpdateEvent) *ConfigSnapshot {
	roots, leaf := TestCerts(t)

	openai := structs.NewServiceName("openai-gpt4", nil)

	policy := &structs.AIGatewayConfigEntry{
		Kind: structs.AIGateway,
		Name: "inference-gateway",
		Processor: structs.AIGatewayProcessor{
			UDSPath:     "/run/consul/ext_proc.sock",
			FailureMode: structs.AIGatewayFailureModeClosed,
		},
		Routing: structs.AIGatewayRouting{
			ConfigValidation: structs.AIGatewayConfigValidationWarn,
			FallbackChain:    []string{"openai-gpt4"},
			MatchRules: []structs.AIGatewayMatchRule{
				{
					When:       structs.AIGatewayMatch{Path: "/v1/chat/completions"},
					Candidates: []string{"openai-gpt4"},
				},
			},
		},
	}

	modelNodes := structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				ID:         "n1",
				Node:       "test1",
				Address:    "10.0.0.1",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Service: "openai-gpt4",
				Address: "10.0.0.1",
				Port:    443,
				AI:      &structs.AIConfig{Role: structs.AIRoleModel},
				Meta:    map[string]string{"provider": "openai", "tier": "premium"},
			},
		},
	}

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: leafWatchID,
			Result:        leaf,
		},
		{
			CorrelationID: meshConfigEntryID,
			Result:        &structs.ConfigEntryResponse{Entry: nil},
		},
		{
			CorrelationID: aiGatewayConfigWatchID,
			Result:        &structs.ConfigEntryResponse{Entry: policy},
		},
		// Intention-driven discovery: the gateway is intention-allowed to reach
		// openai-gpt4, so it discovers it (and starts its health watch) with no
		// routing-policy entry required.
		{
			CorrelationID: intentionUpstreamsID,
			Result:        &structs.IndexedServiceList{Services: structs.ServiceList{openai}},
		},
		{
			CorrelationID: inferenceModelServiceIDPrefix + openai.String(),
			Result:        &structs.IndexedCheckServiceNodes{Nodes: modelNodes},
		},
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindInferenceGateway,
		Service: "inference-gateway",
		Address: "1.2.3.4",
		Port:    8443,
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates))
}

// TestConfigSnapshotInferenceGatewayRateLimit returns an inference gateway snapshot
// that additionally binds a rate-limit StateStore + RateLimit policy: it exercises
// the outbound store listener, the mTLS store cluster + its EDS endpoints, and the
// consul.ai.ratelimit listener metadata / ext_proc forwarding.
func TestConfigSnapshotInferenceGatewayRateLimit(t testing.T, nsFn func(ns *structs.NodeService), extraUpdates []UpdateEvent) *ConfigSnapshot {
	roots, leaf := TestCerts(t)

	openai := structs.NewServiceName("openai-gpt4", nil)

	policy := &structs.AIGatewayConfigEntry{
		Kind: structs.AIGateway,
		Name: "inference-gateway",
		Processor: structs.AIGatewayProcessor{
			UDSPath:     "/run/consul/ext_proc.sock",
			FailureMode: structs.AIGatewayFailureModeClosed,
		},
		Routing: structs.AIGatewayRouting{
			ConfigValidation: structs.AIGatewayConfigValidationWarn,
			FallbackChain:    []string{"openai-gpt4"},
			MatchRules: []structs.AIGatewayMatchRule{
				{
					When:       structs.AIGatewayMatch{Path: "/v1/chat/completions"},
					Candidates: []string{"openai-gpt4"},
				},
			},
		},
		StateStore: &structs.AIGatewayStateStore{
			Service:       "valkey",
			LocalBindPort: 6379,
		},
		RateLimit: &structs.AIGatewayRateLimit{
			Enabled:     true,
			Enforcement: "deny",
			DegradeMode: "fail_closed",
			Dimensions:  []string{"agent", "tier", "global"},
			Default: &structs.AIGatewayLimitPair{
				Requests: &structs.AIGatewayLimit{Count: 60},
				Tokens:   &structs.AIGatewayLimit{Count: 10000},
			},
			Global: &structs.AIGatewayLimitPair{
				Requests: &structs.AIGatewayLimit{Count: 20000},
				Tokens:   &structs.AIGatewayLimit{Count: 1000000, Unit: "day"},
			},
			TierLimits: []structs.AIGatewayTierLimit{
				{Tier: "standard", Requests: &structs.AIGatewayLimit{Count: 100}, Tokens: &structs.AIGatewayLimit{Count: 20000}},
			},
			TierBindings: []structs.AIGatewayTierBinding{
				{Tier: "standard", Partition: "default"},
			},
		},
	}

	modelNodes := structs.CheckServiceNodes{
		{
			Node: &structs.Node{ID: "n1", Node: "test1", Address: "10.0.0.1", Datacenter: "dc1"},
			Service: &structs.NodeService{
				Service: "openai-gpt4",
				Address: "10.0.0.1",
				Port:    443,
				AI:      &structs.AIConfig{Role: structs.AIRoleModel},
				Meta:    map[string]string{"provider": "openai", "tier": "premium"},
			},
		},
	}

	// The store's Connect (sidecar-proxy) endpoint, discovered via the Connect
	// health watch — recorded verbatim (no ai.role filter).
	storeNodes := structs.CheckServiceNodes{
		{
			Node: &structs.Node{ID: "n2", Node: "test2", Address: "10.0.0.9", Datacenter: "dc1"},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "valkey-sidecar-proxy",
				Address: "10.0.0.9",
				Port:    20000,
				Proxy:   structs.ConnectProxyConfig{DestinationServiceName: "valkey"},
			},
		},
	}

	baseEvents := []UpdateEvent{
		{CorrelationID: rootsWatchID, Result: roots},
		{CorrelationID: leafWatchID, Result: leaf},
		{CorrelationID: meshConfigEntryID, Result: &structs.ConfigEntryResponse{Entry: nil}},
		{CorrelationID: aiGatewayConfigWatchID, Result: &structs.ConfigEntryResponse{Entry: policy}},
		{
			CorrelationID: intentionUpstreamsID,
			Result:        &structs.IndexedServiceList{Services: structs.ServiceList{openai}},
		},
		{
			CorrelationID: inferenceModelServiceIDPrefix + openai.String(),
			Result:        &structs.IndexedCheckServiceNodes{Nodes: modelNodes},
		},
		// The store watch is driven by the bound entry's StateStore (reconcileStoreWatch),
		// independent of intention-based model discovery.
		{
			CorrelationID: inferenceStateStoreWatchID,
			Result:        &structs.IndexedCheckServiceNodes{Nodes: storeNodes},
		},
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindInferenceGateway,
		Service: "inference-gateway",
		Address: "1.2.3.4",
		Port:    8443,
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates))
}
