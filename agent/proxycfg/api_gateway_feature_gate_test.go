// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"sync"
	"testing"
	"time"

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

// TestWatchFeatureGates_PublishPropagates verifies the full
// Store.Publish -> Watch closure -> Manager.watchFeatureGates -> proxy state
// invalidation path.
func TestWatchFeatureGates_PublishPropagates(t *testing.T) {
	store := &featuregate.Store{}
	gateway := &state{
		source:          ProxySourceCatalog,
		serviceInstance: serviceInstance{kind: structs.ServiceKindAPIGateway},
		ch:              make(chan UpdateEvent, 1),
		doneCh:          make(chan struct{}),
	}
	m := &Manager{
		ManagerConfig: ManagerConfig{FeatureGate: store},
		proxies: map[ProxyID]*state{
			{NodeName: "catalog-gateway"}: gateway,
		},
		doneCh:           make(chan struct{}),
		featureGateDedup: newFeatureGateDedup(),
	}

	go m.watchFeatureGates()
	time.Sleep(10 * time.Millisecond)

	require.True(t, store.Publish(featuregate.Snapshot{
		StatusIndex: 1,
		Features:    map[string]bool{featuregate.APIGatewayUpstreamRouting.String(): true},
	}))

	select {
	case event := <-gateway.ch:
		require.Equal(t, featureGateWatchID, event.CorrelationID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for feature-gate invalidation event")
	}

	close(m.doneCh)
}

// TestWatchFeatureGates_StalePublishNotPropagated verifies that a stale
// (lower StatusIndex) Publish does not produce a second invalidation.
func TestWatchFeatureGates_StalePublishNotPropagated(t *testing.T) {
	store := &featuregate.Store{}
	require.True(t, store.Publish(featuregate.Snapshot{StatusIndex: 5}))

	gateway := &state{
		source:          ProxySourceCatalog,
		serviceInstance: serviceInstance{kind: structs.ServiceKindAPIGateway},
		ch:              make(chan UpdateEvent, 1),
		doneCh:          make(chan struct{}),
	}
	m := &Manager{
		ManagerConfig: ManagerConfig{FeatureGate: store},
		proxies: map[ProxyID]*state{
			{NodeName: "catalog-gateway"}: gateway,
		},
		doneCh:           make(chan struct{}),
		featureGateDedup: newFeatureGateDedup(),
	}
	go m.watchFeatureGates()

	require.False(t, store.Publish(featuregate.Snapshot{StatusIndex: 3}))

	select {
	case event := <-gateway.ch:
		t.Fatalf("unexpected event from stale publish: %#v", event)
	case <-time.After(200 * time.Millisecond):
	}

	close(m.doneCh)
}

// TestWatchFeatureGates_ClosedProxyStateSkipped verifies that a proxy whose
// doneCh is already closed does not block or receive an event.
func TestWatchFeatureGates_ClosedProxyStateSkipped(t *testing.T) {
	store := &featuregate.Store{}

	stopped := &state{
		source:          ProxySourceCatalog,
		serviceInstance: serviceInstance{kind: structs.ServiceKindAPIGateway},
		ch:              make(chan UpdateEvent),
		doneCh:          make(chan struct{}),
	}
	close(stopped.doneCh)

	active := &state{
		source:          ProxySourceCatalog,
		serviceInstance: serviceInstance{kind: structs.ServiceKindAPIGateway},
		ch:              make(chan UpdateEvent, 1),
		doneCh:          make(chan struct{}),
	}

	m := &Manager{
		ManagerConfig: ManagerConfig{FeatureGate: store},
		proxies: map[ProxyID]*state{
			{NodeName: "stopped-gateway"}: stopped,
			{NodeName: "active-gateway"}:  active,
		},
		doneCh:           make(chan struct{}),
		featureGateDedup: newFeatureGateDedup(),
	}

	go m.watchFeatureGates()
	time.Sleep(10 * time.Millisecond)

	require.True(t, store.Publish(featuregate.Snapshot{StatusIndex: 1}))

	select {
	case event := <-active.ch:
		require.Equal(t, featureGateWatchID, event.CorrelationID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for active gateway invalidation")
	}

	close(m.doneCh)
}

// TestWatchFeatureGates_RapidPublishDoesNotBlock verifies that rapid successive
// publishes do not block the watcher goroutine even when the proxy state
// channel is momentarily full.
func TestWatchFeatureGates_RapidPublishDoesNotBlock(t *testing.T) {
	store := &featuregate.Store{}
	gateway := &state{
		source:          ProxySourceCatalog,
		serviceInstance: serviceInstance{kind: structs.ServiceKindAPIGateway},
		ch:              make(chan UpdateEvent, 1),
		doneCh:          make(chan struct{}),
	}
	m := &Manager{
		ManagerConfig: ManagerConfig{FeatureGate: store},
		proxies: map[ProxyID]*state{
			{NodeName: "catalog-gateway"}: gateway,
		},
		doneCh:           make(chan struct{}),
		featureGateDedup: newFeatureGateDedup(),
	}
	go m.watchFeatureGates()

	const n = 10
	var wg sync.WaitGroup
	for i := uint64(1); i <= n; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			store.Publish(featuregate.Snapshot{StatusIndex: idx})
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchFeatureGates blocked on rapid concurrent publishes")
	}

	close(m.doneCh)
}

// TestWatchFeatureGates_ChannelFullEventuallyDelivers verifies that when the
// proxy state channel is full, a feature-gate invalidation is not lost. The
// existing important event remains first, and the feature-gate update is
// delivered after it without another publish.
func TestWatchFeatureGates_ChannelFullEventuallyDelivers(t *testing.T) {
	store := &featuregate.Store{}
	gateway := &state{
		source:          ProxySourceCatalog,
		serviceInstance: serviceInstance{kind: structs.ServiceKindAPIGateway},
		ch:              make(chan UpdateEvent, 1),
		doneCh:          make(chan struct{}),
	}
	m := &Manager{
		ManagerConfig: ManagerConfig{FeatureGate: store},
		proxies: map[ProxyID]*state{
			{NodeName: "catalog-gateway"}: gateway,
		},
		doneCh:           make(chan struct{}),
		featureGateDedup: newFeatureGateDedup(),
	}

	go m.watchFeatureGates()
	time.Sleep(10 * time.Millisecond)

	important := UpdateEvent{CorrelationID: "important-watch"}
	gateway.ch <- important

	require.True(t, store.Publish(featuregate.Snapshot{StatusIndex: 1}))

	select {
	case event := <-gateway.ch:
		require.Equal(t, "important-watch", event.CorrelationID, "important event should not be discarded")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for important event")
	}

	var featureGateEvent UpdateEvent
	require.Eventually(t, func() bool {
		select {
		case event := <-gateway.ch:
			featureGateEvent = event
			return true
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, featureGateWatchID, featureGateEvent.CorrelationID)

	close(m.doneCh)
}

func TestWatchFeatureGates_ShutdownStopsWatcherAndRefresher(t *testing.T) {
	store := &featuregate.Store{}
	gateway := &state{
		source:          ProxySourceCatalog,
		serviceInstance: serviceInstance{kind: structs.ServiceKindAPIGateway},
		ch:              make(chan UpdateEvent, 1),
		doneCh:          make(chan struct{}),
	}
	watcherManager := &Manager{
		ManagerConfig: ManagerConfig{FeatureGate: store},
		proxies: map[ProxyID]*state{
			{NodeName: "catalog-gateway"}: gateway,
		},
		doneCh:           make(chan struct{}),
		featureGateDedup: newFeatureGateDedup(),
	}

	watchDone := make(chan struct{})
	go func() {
		watcherManager.watchFeatureGates()
		close(watchDone)
	}()

	refresherManager := &Manager{
		doneCh:           make(chan struct{}),
		featureGateDedup: newFeatureGateDedup(),
	}
	refresherDone := make(chan struct{})
	go func() {
		refresherManager.featureGateRefresher()
		close(refresherDone)
	}()

	close(watcherManager.doneCh)
	close(refresherManager.doneCh)

	select {
	case <-watchDone:
	case <-time.After(2 * time.Second):
		t.Fatal("watchFeatureGates did not exit after close")
	}

	select {
	case <-refresherDone:
	case <-time.After(2 * time.Second):
		t.Fatal("featureGateRefresher did not exit after close")
	}
}
