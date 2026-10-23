// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

// connect_proxy_feature_gate_test.go tests the handlerConnectProxy
// handleUpdate case for featureGateWatchID, covering:
//   - no-op when the effective value is unchanged
//   - snap.LocalizedDNSEnabled flips when the gate changes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

func newConnectProxyHandler(gate featuregate.Gate) *handlerConnectProxy {
	return &handlerConnectProxy{
		handlerState: handlerState{
			stateConfig: stateConfig{
				logger:      hclog.NewNullLogger(),
				featureGate: gate,
			},
			ch: make(chan UpdateEvent, 10),
		},
	}
}

func TestHandlerConnectProxy_HandleUpdate_FeatureGateNoOp(t *testing.T) {
	store := &featuregate.Store{}
	handler := newConnectProxyHandler(store)
	snap := &ConfigSnapshot{Kind: structs.ServiceKindConnectProxy}
	snap.LocalizedDNSEnabled = false // matches gate (disabled)

	event := UpdateEvent{CorrelationID: featureGateWatchID}
	err := handler.handleUpdate(context.Background(), event, snap)
	require.NoError(t, err)
	require.False(t, snap.LocalizedDNSEnabled)
}

func TestHandlerConnectProxy_HandleUpdate_FeatureGateEnabled(t *testing.T) {
	store := &featuregate.Store{}
	require.True(t, store.Publish(featuregate.Snapshot{
		StatusIndex: 1,
		Features: map[string]bool{
			featuregate.LocalizedDNS.String(): true,
		},
	}))

	handler := newConnectProxyHandler(store)
	snap := &ConfigSnapshot{Kind: structs.ServiceKindConnectProxy}
	snap.LocalizedDNSEnabled = false

	event := UpdateEvent{CorrelationID: featureGateWatchID}
	err := handler.handleUpdate(context.Background(), event, snap)
	require.NoError(t, err)
	require.True(t, snap.LocalizedDNSEnabled)
}

func TestHandlerConnectProxy_HandleUpdate_FeatureGateDisabled(t *testing.T) {
	store := &featuregate.Store{}
	require.True(t, store.Publish(featuregate.Snapshot{StatusIndex: 1}))

	handler := newConnectProxyHandler(store)
	snap := &ConfigSnapshot{Kind: structs.ServiceKindConnectProxy}
	snap.LocalizedDNSEnabled = true // was enabled, gate now disabled

	event := UpdateEvent{CorrelationID: featureGateWatchID}
	err := handler.handleUpdate(context.Background(), event, snap)
	require.NoError(t, err)
	require.False(t, snap.LocalizedDNSEnabled)
}
