// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
)

func featureGateReply(index uint64, enabled bool, lastContact time.Duration) *structs.FeatureGateQueryResponse {
	return &structs.FeatureGateQueryResponse{
		QueryMeta: structs.QueryMeta{Index: index, LastContact: lastContact},
		Features:  []structs.FeatureGateInfo{{Name: featuregate.LocalizedDNS.String(), EffectiveEnabled: enabled}},
	}
}

func TestPublishClientFeatureGates_LaggingFollower(t *testing.T) {
	store := &featuregate.Store{}
	gate := featuregate.LocalizedDNS
	follower := time.Second

	publishClientFeatureGates(store, featureGateReply(50, true, 0))
	require.True(t, store.Enabled(gate))

	watch := store.Watch()
	publishClientFeatureGates(store, featureGateReply(30, false, follower))
	require.True(t, store.Enabled(gate), "an older generation must not roll the cache back")
	require.Equal(t, uint64(50), store.Current().StatusIndex)
	select {
	case <-watch:
		t.Fatal("an ignored response must not notify watchers")
	default:
	}

	publishClientFeatureGates(store, &structs.FeatureGateQueryResponse{Uninitialized: true, QueryMeta: structs.QueryMeta{LastContact: follower}})
	require.True(t, store.Enabled(gate), "a follower that has not replayed gate state must not clear the cache")

	publishClientFeatureGates(store, &structs.FeatureGateQueryResponse{Uninitialized: true})
	require.False(t, store.Enabled(gate), "the leader is authoritative that no gate state exists")
}

func TestRefreshClientFeatureGates(t *testing.T) {
	var request *structs.FeatureGateQueryRequest
	var next *structs.FeatureGateQueryResponse
	delegate := &delegateMock{}
	delegate.On("RPC", "Operator.FeatureGateGet", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			request = args.Get(1).(*structs.FeatureGateQueryRequest)
			*args.Get(2).(*structs.FeatureGateQueryResponse) = *next
		}).Return(nil)
	t.Cleanup(func() { delegate.AssertExpectations(t) })
	a := &Agent{
		config:   &config.RuntimeConfig{Datacenter: "dc1"},
		tokens:   new(token.Store),
		delegate: delegate,
	}
	initializeFeatureGateHTTPTestAgent(t, a)
	store := &featuregate.Store{}
	gate := featuregate.LocalizedDNS

	next = featureGateReply(50, true, time.Second)
	index, err := a.refreshClientFeatureGates(store, 0, true)
	require.NoError(t, err)
	require.True(t, request.AllowStale, "the long poll must be servable by followers")
	require.Equal(t, uint64(50), index)
	require.True(t, store.Enabled(gate))

	next = featureGateReply(30, false, time.Second)
	index, err = a.refreshClientFeatureGates(store, index, true)
	require.NoError(t, err)
	require.Equal(t, uint64(50), request.MinQueryIndex)
	require.Equal(t, uint64(50), index, "the poll index must not move back to a lagging follower's")
	require.True(t, store.Enabled(gate))

	next = featureGateReply(60, false, time.Second)
	index, err = a.refreshClientFeatureGates(store, index, true)
	require.NoError(t, err)
	require.Equal(t, uint64(60), index)
	require.False(t, store.Enabled(gate), "a newer generation must be installed")
}

func TestPublishClientFeatureGates(t *testing.T) {
	store := &featuregate.Store{}
	gate := featuregate.LocalizedDNS
	watch := store.Watch()
	publishClientFeatureGates(store, &structs.FeatureGateQueryResponse{
		QueryMeta: structs.QueryMeta{Index: 10},
		Features:  []structs.FeatureGateInfo{{Name: gate.String(), EffectiveEnabled: false}},
	})
	select {
	case <-watch:
	default:
		t.Fatal("disabled policy did not notify watchers")
	}
	require.False(t, store.Enabled(gate))

	publishClientFeatureGates(store, &structs.FeatureGateQueryResponse{
		QueryMeta: structs.QueryMeta{Index: 11},
		Features:  []structs.FeatureGateInfo{{Name: gate.String(), EffectiveEnabled: true}},
	})
	require.True(t, store.Enabled(gate))

	publishClientFeatureGates(store, &structs.FeatureGateQueryResponse{Uninitialized: true})
	require.False(t, store.Enabled(gate))

	publishClientFeatureGates(store, &structs.FeatureGateQueryResponse{
		QueryMeta: structs.QueryMeta{Index: 1},
		Features:  []structs.FeatureGateInfo{{Name: gate.String(), EffectiveEnabled: true}},
	})
	require.True(t, store.Enabled(gate))
	publishClientFeatureGates(store, &structs.FeatureGateQueryResponse{
		QueryMeta: structs.QueryMeta{Index: 1},
		Features:  []structs.FeatureGateInfo{{Name: gate.String(), EffectiveEnabled: false}},
	})
	require.True(t, store.Enabled(gate), "an equal-index response must not replace committed state")
}
