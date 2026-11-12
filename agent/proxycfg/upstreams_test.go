// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
)

// TestHandleUpdateUpstreams_DiscoveryChainWatchReset asserts that target watches
// are only torn down when the compiled discovery chain actually changed.
//
// The discovery chain watch wakes on any config-entry write in the cluster
// because the backing memdb index spans the whole config-entries table, so most
// wakeups redeliver an identical chain. Resetting on those cancels every target
// watch and clears WatchedUpstreamEndpoints, which transiently leaves clusters
// without their endpoint assignments.
func TestHandleUpdateUpstreams_DiscoveryChainWatchReset(t *testing.T) {
	const (
		targetID  = "db.default.default.dc1"
		chainName = "db"
	)

	compileChain := func(t *testing.T, entries ...structs.ConfigEntry) *structs.CompiledDiscoveryChain {
		t.Helper()
		return discoverychain.TestCompileConfigEntries(
			t, chainName, "default", "default", "dc1", "trustdomain.consul", nil,
			discoChainSetWithEntries(entries...),
		)
	}

	// setup returns a handler and snapshot that have already processed an initial
	// discovery chain update, so the upstream's target watches are established.
	setup := func(t *testing.T) (*handlerUpstreams, *ConfigSnapshot, UpstreamID, *structs.CompiledDiscoveryChain) {
		t.Helper()

		upstream := &structs.Upstream{DestinationName: chainName}
		uid := NewUpstreamID(upstream)

		config := stateConfig{logger: hclog.NewNullLogger(), source: &structs.QuerySource{Datacenter: "dc1"}}
		recordWatches(&config)

		snap := &ConfigSnapshot{
			Kind: structs.ServiceKindConnectProxy,
			ConnectProxy: configSnapshotConnectProxy{
				ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
					UpstreamConfig:           map[UpstreamID]*structs.Upstream{uid: upstream},
					DiscoveryChain:           make(map[UpstreamID]*structs.CompiledDiscoveryChain),
					WatchedUpstreams:         make(map[UpstreamID]map[string]context.CancelFunc),
					WatchedUpstreamEndpoints: make(map[UpstreamID]map[string]structs.CheckServiceNodes),
					WatchedGateways:          make(map[UpstreamID]map[string]context.CancelFunc),
					WatchedGatewayEndpoints:  make(map[UpstreamID]map[string]structs.CheckServiceNodes),
					// Make the upstream implicit so the chain update is not
					// discarded as belonging to an unknown upstream.
					IntentionUpstreams: map[UpstreamID]struct{}{uid: {}},
				},
			},
		}

		handler := &handlerUpstreams{handlerState: handlerState{
			stateConfig: config,
			serviceInstance: serviceInstance{
				kind:     structs.ServiceKindConnectProxy,
				proxyCfg: structs.ConnectProxyConfig{Mode: structs.ProxyModeDefault},
			},
			ch: make(chan UpdateEvent, 10),
		}}

		chain := compileChain(t)
		require.NoError(t, handler.handleUpdateUpstreams(context.Background(), UpdateEvent{
			CorrelationID: "discovery-chain:" + uid.String(),
			Result:        &structs.DiscoveryChainResponse{Chain: chain},
		}, snap))

		require.Contains(t, snap.ConnectProxy.WatchedUpstreams[uid], targetID,
			"initial update should establish the target watch")

		return handler, snap, uid, chain
	}

	// armResetDetector replaces the stored cancel funcs with sentinels and seeds
	// endpoint data, so a subsequent reset is directly observable.
	armResetDetector := func(snap *ConfigSnapshot, uid UpstreamID) *bool {
		reset := false
		for id := range snap.ConnectProxy.WatchedUpstreams[uid] {
			snap.ConnectProxy.WatchedUpstreams[uid][id] = func() { reset = true }
		}
		snap.ConnectProxy.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{
			targetID: {},
		}
		return &reset
	}

	sendChain := func(t *testing.T, handler *handlerUpstreams, snap *ConfigSnapshot, uid UpstreamID, chain *structs.CompiledDiscoveryChain) {
		t.Helper()
		require.NoError(t, handler.handleUpdateUpstreams(context.Background(), UpdateEvent{
			CorrelationID: "discovery-chain:" + uid.String(),
			Result:        &structs.DiscoveryChainResponse{Chain: chain},
		}, snap))
	}

	t.Run("unchanged chain does not reset watches", func(t *testing.T) {
		handler, snap, uid, chain := setup(t)
		reset := armResetDetector(snap, uid)

		// Recompile the same config entries: a distinct object with identical
		// content, which is what a spurious wakeup delivers.
		sendChain(t, handler, snap, uid, compileChain(t))

		require.False(t, *reset, "identical chain must not cancel target watches")
		require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints[uid], targetID,
			"identical chain must not clear watched endpoints")
		require.Equal(t, chain.GetHash(), snap.ConnectProxy.DiscoveryChain[uid].GetHash())
	})

	t.Run("changed chain resets watches", func(t *testing.T) {
		handler, snap, uid, _ := setup(t)
		reset := armResetDetector(snap, uid)

		changed := compileChain(t, &structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           chainName,
			ConnectTimeout: 1234,
		})
		sendChain(t, handler, snap, uid, changed)

		require.True(t, *reset, "changed chain must cancel existing target watches")
		require.Equal(t, changed.GetHash(), snap.ConnectProxy.DiscoveryChain[uid].GetHash())
	})

	t.Run("unchanged chain repairs a missing watch", func(t *testing.T) {
		handler, snap, uid, _ := setup(t)
		armResetDetector(snap, uid)

		// resetWatchesFromChain can fail partway through and the error is
		// discarded by the caller, so an unchanged chain must still repair an
		// incomplete watch set rather than skipping indefinitely.
		delete(snap.ConnectProxy.WatchedUpstreams[uid], targetID)

		sendChain(t, handler, snap, uid, compileChain(t))

		require.Contains(t, snap.ConnectProxy.WatchedUpstreams[uid], targetID,
			"missing watch must be re-established even when the chain is unchanged")
	})

	t.Run("first chain establishes watches", func(t *testing.T) {
		_, snap, uid, _ := setup(t)
		require.Contains(t, snap.ConnectProxy.WatchedUpstreams[uid], targetID)
	})
}
