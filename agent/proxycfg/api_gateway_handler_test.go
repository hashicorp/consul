// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

// api_gateway_handle_feature_gate_test.go tests the handlerAPIGateway
// handleUpdate case for featureGateWatchID, covering:
//   - no-op when the effective value is unchanged
//   - snap.APIGateway.ComposeUpstreamRouting flips when gate changes
//   - WatchedDiscoveryChains are cancelled on gate change (refreshed)
//   - recompileDiscoveryChains is called after the gate change

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

// newAgentlessGatewayHandler creates a handlerAPIGateway configured as
// agentless with the supplied Gate.
func newAgentlessGatewayHandler(gate featuregate.Gate) *handlerAPIGateway {
	return &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				logger:      hclog.NewNullLogger(),
				featureGate: gate,
				agentless:   true,
			},
			ch: make(chan UpdateEvent, 10),
		},
	}
}

// minimalAPIGatewaySnap returns a ConfigSnapshot with the APIGateway fields
// required by handleUpdate and refreshRouteDiscoveryChainWatches initialised
// to non-nil values so method calls do not panic.
func minimalAPIGatewaySnap() *ConfigSnapshot {
	return &ConfigSnapshot{
		Kind: structs.ServiceKindAPIGateway,
		APIGateway: configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain:           make(map[UpstreamID]*structs.CompiledDiscoveryChain),
				WatchedDiscoveryChains:   make(map[UpstreamID]context.CancelFunc),
				WatchedUpstreams:         make(map[UpstreamID]map[string]context.CancelFunc),
				WatchedUpstreamEndpoints: make(map[UpstreamID]map[string]structs.CheckServiceNodes),
				WatchedGateways:          make(map[UpstreamID]map[string]context.CancelFunc),
				WatchedGatewayEndpoints:  make(map[UpstreamID]map[string]structs.CheckServiceNodes),
				UpstreamPeerTrustBundles: watch.NewMap[PeerName, *pbpeering.PeeringTrustBundle](),
				WatchedLocalGWEndpoints:  watch.NewMap[string, structs.CheckServiceNodes](),
			},
			Upstreams:              make(listenerRouteUpstreams),
			UpstreamsSet:           make(routeUpstreamSet),
			HTTPRoutes:             watch.NewMap[structs.ResourceReference, *structs.HTTPRouteConfigEntry](),
			TCPRoutes:              watch.NewMap[structs.ResourceReference, *structs.TCPRouteConfigEntry](),
			InlineCertificates:     watch.NewMap[structs.ResourceReference, *structs.InlineCertificateConfigEntry](),
			FileSystemCertificates: watch.NewMap[structs.ResourceReference, *structs.FileSystemCertificateConfigEntry](),
			Listeners:              make(map[string]structs.APIGatewayListener),
			BoundListeners:         make(map[string]structs.BoundAPIGatewayListener),
		},
	}
}

// ---------------------------------------------------------------------------
// handleUpdate – no-op when effective value is unchanged
// ---------------------------------------------------------------------------

func TestHandlerAPIGateway_HandleUpdate_FeatureGateNoOp(t *testing.T) {
	store := &featuregate.Store{}
	// Gate is disabled (zero value) — ComposeUpstreamRouting starts false.
	handler := newAgentlessGatewayHandler(store)
	snap := minimalAPIGatewaySnap()
	snap.APIGateway.ComposeUpstreamRouting = false // matches gate (disabled)

	// Inject a cancel into WatchedDiscoveryChains so we can detect if it was
	// erroneously cancelled.
	cancelled := false
	snap.APIGateway.WatchedDiscoveryChains[UpstreamID{}] = func() { cancelled = true }

	event := UpdateEvent{CorrelationID: featureGateWatchID}
	err := handler.handleUpdate(context.Background(), event, snap)
	require.NoError(t, err)

	// No change in gate → ComposeUpstreamRouting unchanged.
	require.False(t, snap.APIGateway.ComposeUpstreamRouting)
	// The existing watch must NOT have been cancelled.
	require.False(t, cancelled, "no-op: existing discovery chain watch must not be cancelled")
}

// ---------------------------------------------------------------------------
// handleUpdate – gate flips from disabled → enabled
// ---------------------------------------------------------------------------

func TestHandlerAPIGateway_HandleUpdate_FeatureGateEnabled(t *testing.T) {
	store := &featuregate.Store{}
	// Enable the gate.
	store.Publish(featuregate.Snapshot{
		StatusIndex: 1,
		Features:    map[string]bool{featuregate.APIGatewayUpstreamRouting.String(): true},
	})

	handler := newAgentlessGatewayHandler(store)
	snap := minimalAPIGatewaySnap()
	// Current snapshot has gate=false; gate is now enabled → should change.
	snap.APIGateway.ComposeUpstreamRouting = false

	// Inject a cancel to confirm it is called (chain refresh).
	cancelCalled := false
	uid := UpstreamID{Name: "my-service"}
	snap.APIGateway.WatchedDiscoveryChains[uid] = func() { cancelCalled = true }

	event := UpdateEvent{CorrelationID: featureGateWatchID}
	err := handler.handleUpdate(context.Background(), event, snap)
	require.NoError(t, err)

	require.True(t, snap.APIGateway.ComposeUpstreamRouting,
		"ComposeUpstreamRouting must be true after gate becomes enabled")
	require.True(t, cancelCalled,
		"existing discovery chain watch must be cancelled when gate changes")
}

// ---------------------------------------------------------------------------
// handleUpdate – gate flips from enabled → disabled
// ---------------------------------------------------------------------------

func TestHandlerAPIGateway_HandleUpdate_FeatureGateDisabled(t *testing.T) {
	// Start with gate enabled.
	store := &featuregate.Store{}
	store.Publish(featuregate.Snapshot{
		StatusIndex: 1,
		Features:    map[string]bool{featuregate.APIGatewayUpstreamRouting.String(): true},
	})

	handler := newAgentlessGatewayHandler(store)
	snap := minimalAPIGatewaySnap()
	snap.APIGateway.ComposeUpstreamRouting = true // was enabled

	// Now disable the gate by publishing an empty snapshot.
	store.Publish(featuregate.Snapshot{StatusIndex: 2})

	cancelCalled := false
	uid := UpstreamID{Name: "svc"}
	snap.APIGateway.WatchedDiscoveryChains[uid] = func() { cancelCalled = true }

	event := UpdateEvent{CorrelationID: featureGateWatchID}
	err := handler.handleUpdate(context.Background(), event, snap)
	require.NoError(t, err)

	require.False(t, snap.APIGateway.ComposeUpstreamRouting,
		"ComposeUpstreamRouting must be false after gate becomes disabled")
	require.True(t, cancelCalled,
		"existing discovery chain watch must be cancelled when gate changes")
}

// ---------------------------------------------------------------------------
// handleUpdate – agentful gateway ignores gate (composeUpstreamRoutingEnabled=false)
// ---------------------------------------------------------------------------

func TestHandlerAPIGateway_HandleUpdate_AgentfulIgnoresGate(t *testing.T) {
	store := &featuregate.Store{}
	store.Publish(featuregate.Snapshot{
		StatusIndex: 1,
		Features:    map[string]bool{featuregate.APIGatewayUpstreamRouting.String(): true},
	})

	// agentless=false → composeUpstreamRoutingEnabled() always returns false.
	handler := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				logger:      hclog.NewNullLogger(),
				featureGate: store,
				agentless:   false,
			},
			ch: make(chan UpdateEvent, 10),
		},
	}
	snap := minimalAPIGatewaySnap()
	snap.APIGateway.ComposeUpstreamRouting = false // should stay false

	cancelCalled := false
	snap.APIGateway.WatchedDiscoveryChains[UpstreamID{Name: "svc"}] = func() { cancelCalled = true }

	event := UpdateEvent{CorrelationID: featureGateWatchID}
	err := handler.handleUpdate(context.Background(), event, snap)
	require.NoError(t, err)

	// agentful: gate effectively disabled → no change, no cancel.
	require.False(t, snap.APIGateway.ComposeUpstreamRouting)
	require.False(t, cancelCalled, "agentful gateway must not cancel watches on gate update")
}

// ---------------------------------------------------------------------------
// refreshRouteDiscoveryChainWatches – cancels existing watches
// ---------------------------------------------------------------------------

func TestRefreshRouteDiscoveryChainWatches_CancelsAllExistingWatches(t *testing.T) {
	store := &featuregate.Store{}
	handler := newAgentlessGatewayHandler(store)
	snap := minimalAPIGatewaySnap()

	// Inject multiple cancel funcs.
	cancelCount := 0
	for i := 0; i < 3; i++ {
		uid := UpstreamID{Name: fmt.Sprintf("svc-%d", i)}
		snap.APIGateway.WatchedDiscoveryChains[uid] = func() { cancelCount++ }
	}

	err := handler.refreshRouteDiscoveryChainWatches(context.Background(), snap)
	require.NoError(t, err)
	require.Equal(t, 3, cancelCount, "all three chain watches should be cancelled")
	require.Empty(t, snap.APIGateway.WatchedDiscoveryChains,
		"WatchedDiscoveryChains should be cleared after refresh")
}

// ---------------------------------------------------------------------------
// composeUpstreamRoutingEnabled – gate nil is safe (fail-closed)
// ---------------------------------------------------------------------------

func TestHandlerAPIGateway_ComposeUpstreamRoutingEnabled_NilGate(t *testing.T) {
	handler := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				featureGate: nil,
				agentless:   true,
			},
		},
	}
	require.False(t, handler.composeUpstreamRoutingEnabled(),
		"nil gate must be fail-closed (returns false)")
}
