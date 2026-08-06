// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

func TestHandlerAPIGateway_ComposeUpstreamRoutingEnabled_AgentlessOnly(t *testing.T) {
	store := &featuregate.Store{}
	require.True(t, store.Publish(featuregate.Snapshot{
		StatusIndex: 1,
		Features: map[string]bool{
			featuregate.APIGatewayUpstreamRouting.String(): true,
		},
	}))

	agentless := &handlerAPIGateway{handlerState: handlerState{stateConfig: stateConfig{
		featureGate: store,
		agentless:   true,
	}}}
	require.True(t, agentless.composeUpstreamRoutingEnabled())

	agentful := &handlerAPIGateway{handlerState: handlerState{stateConfig: stateConfig{
		featureGate: store,
		agentless:   false,
	}}}
	require.False(t, agentful.composeUpstreamRoutingEnabled())

	require.True(t, store.Publish(featuregate.Snapshot{StatusIndex: 2}))
	require.False(t, agentless.composeUpstreamRoutingEnabled())
}

func TestManager_RefreshFeatureGates_AgentlessAPIGatewayOnly(t *testing.T) {
	newTestState := func(source ProxySource, kind structs.ServiceKind) *state {
		return &state{
			source:          source,
			serviceInstance: serviceInstance{kind: kind},
			ch:              make(chan UpdateEvent, 1),
			doneCh:          make(chan struct{}),
		}
	}
	agentlessGateway := newTestState(ProxySourceCatalog, structs.ServiceKindAPIGateway)
	agentfulGateway := newTestState(ProxySourceLocal, structs.ServiceKindAPIGateway)
	agentlessSidecar := newTestState(ProxySourceCatalog, structs.ServiceKindConnectProxy)
	m := &Manager{proxies: map[ProxyID]*state{
		{NodeName: "catalog-gateway"}: agentlessGateway,
		{NodeName: "local-gateway"}:   agentfulGateway,
		{NodeName: "catalog-sidecar"}: agentlessSidecar,
	}}

	m.refreshFeatureGates()
	require.Equal(t, featureGateWatchID, (<-agentlessGateway.ch).CorrelationID)
	for _, unaffected := range []*state{agentfulGateway, agentlessSidecar} {
		select {
		case event := <-unaffected.ch:
			t.Fatalf("unexpected invalidation event: %#v", event)
		default:
		}
	}
}
