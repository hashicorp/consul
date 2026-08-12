// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

func TestPopulateFeatureGateSetResponse_EmptyFeatures(t *testing.T) {
	op := &Operator{srv: &Server{featureGateRegistry: featuregate.DefaultRegistry()}}
	reply := &structs.FeatureGateSetResponse{}
	policy := &structs.FeatureGatePolicy{}
	// A status with an empty Features map causes featureGateInfo to fail with
	// "missing from committed status" before the empty-response guard is hit.
	status := &structs.FeatureGateStatus{Features: map[string]structs.ResolvedFeatureGate{}}

	err := op.populateFeatureGateSetResponse(reply, true, featuregate.APIGatewayUpstreamRouting.String(), policy, status)
	require.EqualError(t, err, fmt.Sprintf("feature gate %q is missing from committed status", featuregate.APIGatewayUpstreamRouting.String()))
}

func TestPopulateFeatureGateSetResponse(t *testing.T) {
	name := featuregate.APIGatewayUpstreamRouting.String()
	op := &Operator{srv: &Server{featureGateRegistry: featuregate.DefaultRegistry()}}
	reply := &structs.FeatureGateSetResponse{}
	policy := &structs.FeatureGatePolicy{RaftIndex: structs.RaftIndex{ModifyIndex: 9}}
	status := &structs.FeatureGateStatus{
		RaftIndex: structs.RaftIndex{ModifyIndex: 11},
		Features: map[string]structs.ResolvedFeatureGate{
			name: {
				DesiredEnabled:   true,
				EffectiveEnabled: true,
				Eligible:         true,
				Source:           string(structs.FeatureGateSourceOperator),
				Reason:           structs.FeatureGateReasonOperatorEnabled,
			},
		},
	}

	require.NoError(t, op.populateFeatureGateSetResponse(reply, true, name, policy, status))
	require.True(t, reply.Applied)
	require.Equal(t, name, reply.Feature.Name)
	require.True(t, reply.Feature.EffectiveEnabled)
	require.Equal(t, uint64(9), reply.Feature.PolicyIndex)
	require.Equal(t, uint64(11), reply.Feature.StatusIndex)
}

func TestFeatureGateInfos_UnknownName(t *testing.T) {
	_, err := featureGateInfos(featuregate.DefaultRegistry(), &structs.FeatureGatePolicy{}, &structs.FeatureGateStatus{}, "does-not-exist")
	require.EqualError(t, err, `unknown feature gate "does-not-exist"`)
}

func TestFeatureGateInfos_SingleFeature(t *testing.T) {
	policy := &structs.FeatureGatePolicy{RaftIndex: structs.RaftIndex{ModifyIndex: 9}}
	status := &structs.FeatureGateStatus{
		RaftIndex: structs.RaftIndex{ModifyIndex: 11},
		Features: map[string]structs.ResolvedFeatureGate{
			featuregate.APIGatewayUpstreamRouting.String(): {
				DesiredEnabled:   true,
				EffectiveEnabled: true,
				Eligible:         true,
				Source:           string(structs.FeatureGateSourceOperator),
				Reason:           structs.FeatureGateReasonOperatorEnabled,
			},
		},
	}

	features, err := featureGateInfos(featuregate.DefaultRegistry(), policy, status, featuregate.APIGatewayUpstreamRouting.String())
	require.NoError(t, err)
	require.Len(t, features, 1)
	require.Equal(t, featuregate.APIGatewayUpstreamRouting.String(), features[0].Name)
	require.Equal(t, uint64(9), features[0].PolicyIndex)
	require.Equal(t, uint64(11), features[0].StatusIndex)
}

func TestFeatureGateInfos_ListAll(t *testing.T) {
	registry := featuregate.DefaultRegistry()
	policy := &structs.FeatureGatePolicy{RaftIndex: structs.RaftIndex{ModifyIndex: 9}}
	status := &structs.FeatureGateStatus{
		RaftIndex: structs.RaftIndex{ModifyIndex: 11},
		Features:  make(map[string]structs.ResolvedFeatureGate),
	}
	for _, definition := range registry.Definitions() {
		status.Features[definition.Name] = structs.ResolvedFeatureGate{
			DesiredEnabled:   true,
			EffectiveEnabled: true,
			Eligible:         true,
			Source:           string(structs.FeatureGateSourceOperator),
			Reason:           structs.FeatureGateReasonOperatorEnabled,
		}
	}

	features, err := featureGateInfos(registry, policy, status, "")
	require.NoError(t, err)
	require.Len(t, features, len(registry.Definitions()))
	for i, definition := range registry.Definitions() {
		require.Equal(t, definition.Name, features[i].Name)
		require.Equal(t, string(definition.Stage), features[i].Stage)
		require.Equal(t, definition.MinVersion.String(), features[i].MinVersion)
		require.Equal(t, definition.Description, features[i].Description)
		require.Equal(t, definition.Owner, features[i].Owner)
		require.True(t, features[i].DesiredEnabled)
		require.True(t, features[i].EffectiveEnabled)
		require.True(t, features[i].Eligible)
		require.Equal(t, uint64(9), features[i].PolicyIndex)
		require.Equal(t, uint64(11), features[i].StatusIndex)
	}
}

func TestFeatureGateInfos_MissingResolvedFeature(t *testing.T) {
	policy := &structs.FeatureGatePolicy{}
	status := &structs.FeatureGateStatus{Features: map[string]structs.ResolvedFeatureGate{}}

	_, err := featureGateInfos(featuregate.DefaultRegistry(), policy, status, featuregate.APIGatewayUpstreamRouting.String())
	require.EqualError(t, err, fmt.Sprintf("feature gate %q is missing from committed status", featuregate.APIGatewayUpstreamRouting.String()))
}
