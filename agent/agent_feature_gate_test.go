package agent

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

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
