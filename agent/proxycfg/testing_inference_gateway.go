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
