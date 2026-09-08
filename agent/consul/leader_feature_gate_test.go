// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"

	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

func TestEnsureFeatureGateFrameworkVersion_UsesPersistedSystemMetadata(t *testing.T) {
	for name, tc := range map[string]struct {
		value       string
		expectReady bool
		expectError bool
	}{
		"required version": {value: structs.SystemMetadataFeatureGatesVersionValue, expectReady: true},
		"newer version":    {value: "2.2.0", expectReady: true},
		"invalid version":  {value: "not-a-version", expectError: true},
	} {
		t.Run(name, func(t *testing.T) {
			store := state.NewStateStore(nil)
			require.NoError(t, store.SystemMetadataSet(1, &structs.SystemMetadataEntry{
				Key:   structs.SystemMetadataFeatureGatesVersionKey,
				Value: tc.value,
			}))
			serverFSM := fsm.NewFromDeps(fsm.Deps{
				Logger:         hclog.NewNullLogger(),
				NewStateStore:  func() *state.Store { return store },
				StorageBackend: fsm.NullStorageBackend,
			})
			server := &Server{fsm: serverFSM}

			ready, err := server.ensureFeatureGateFrameworkVersion()
			if tc.expectError {
				require.Error(t, err)
				require.False(t, ready)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectReady, ready)
		})
	}
}

func TestResolveFeatureGateStatus(t *testing.T) {
	policy := &structs.FeatureGatePolicy{
		Settings: map[string]structs.FeatureGateSetting{
			featuregate.APIGatewayUpstreamRouting.String(): {
				Enabled: true,
				Source:  structs.FeatureGateSourceOperator,
			},
		},
		RaftIndex: structs.RaftIndex{ModifyIndex: 7},
	}

	status := resolveFeatureGateStatus(featuregate.DefaultRegistry(), policy, func(*version.Version) (bool, bool) {
		return true, true
	})
	resolved := status.Features[featuregate.APIGatewayUpstreamRouting.String()]
	require.Equal(t, uint64(7), status.PolicyIndex)
	require.True(t, resolved.DesiredEnabled)
	require.True(t, resolved.EffectiveEnabled)
	require.True(t, resolved.Eligible)
	require.Equal(t, string(featuregate.SourceOperator), resolved.Source)
	require.Equal(t, structs.FeatureGateReasonOperatorEnabled, resolved.Reason)

	// meetsMinimum=true but membersFound=false → no-server-version-data
	status = resolveFeatureGateStatus(featuregate.DefaultRegistry(), policy, func(*version.Version) (bool, bool) {
		return true, false
	})
	resolved = status.Features[featuregate.APIGatewayUpstreamRouting.String()]
	require.True(t, resolved.DesiredEnabled)
	require.False(t, resolved.EffectiveEnabled)
	require.False(t, resolved.Eligible)
	require.Equal(t, structs.FeatureGateReasonNoServerVersionData, resolved.Reason)

	// meetsMinimum=false and membersFound=true → below-minimum-version
	status = resolveFeatureGateStatus(featuregate.DefaultRegistry(), policy, func(*version.Version) (bool, bool) {
		return false, true
	})
	resolved = status.Features[featuregate.APIGatewayUpstreamRouting.String()]
	require.True(t, resolved.DesiredEnabled)
	require.False(t, resolved.EffectiveEnabled)
	require.False(t, resolved.Eligible)
	require.Equal(t, structs.FeatureGateReasonBelowMinimumVersion, resolved.Reason)
}

func TestResolveFeatureGateStatus_PreservesUnknownPolicyEntries(t *testing.T) {
	// A policy that has a setting for a feature name absent from the registry
	// (simulates a rolling downgrade where the new leader has a smaller registry).
	policy := &structs.FeatureGatePolicy{
		Settings: map[string]structs.FeatureGateSetting{
			featuregate.APIGatewayUpstreamRouting.String(): {
				Enabled: true,
				Source:  structs.FeatureGateSourceOperator,
			},
			"future-ent-feature": {
				Enabled: true,
				Source:  structs.FeatureGateSourceOperator,
			},
		},
		RaftIndex: structs.RaftIndex{ModifyIndex: 9},
	}

	status := resolveFeatureGateStatus(featuregate.DefaultRegistry(), policy, func(*version.Version) (bool, bool) {
		return true, true
	})

	// Known feature is resolved normally.
	known := status.Features[featuregate.APIGatewayUpstreamRouting.String()]
	require.True(t, known.EffectiveEnabled)

	// Unknown feature is present in status, fail-closed, with intent preserved.
	unknown, ok := status.Features["future-ent-feature"]
	require.True(t, ok, "unknown policy entry must be carried through to status")
	require.False(t, unknown.EffectiveEnabled, "unknown feature must be fail-closed")
	require.False(t, unknown.Eligible)
	require.True(t, unknown.DesiredEnabled, "operator intent must be preserved")
	require.Equal(t, string(structs.FeatureGateSourceOperator), unknown.Source)
	require.Equal(t, structs.FeatureGateReasonUnknownFeature, unknown.Reason)
}

func TestFeatureGateStatusesEqualIgnoresRaftIndexes(t *testing.T) {
	left := &structs.FeatureGateStatus{
		PolicyIndex:    5,
		RegistryDigest: "digest",
		Features:       map[string]structs.ResolvedFeatureGate{"feature": {EffectiveEnabled: true}},
		RaftIndex:      structs.RaftIndex{CreateIndex: 5, ModifyIndex: 10},
	}
	right := left.Clone()
	right.CreateIndex = 20
	right.ModifyIndex = 20
	require.True(t, featureGateStatusesEqual(left, right))

	right.Features["feature"] = structs.ResolvedFeatureGate{}
	require.False(t, featureGateStatusesEqual(left, right))
}

func TestBootstrapMismatches(t *testing.T) {
	committed := &structs.FeatureGatePolicy{
		Settings: map[string]structs.FeatureGateSetting{
			"feature-a": {Enabled: true, Source: structs.FeatureGateSourceBootstrap},
			"feature-b": {Enabled: false, Source: structs.FeatureGateSourceOperator},
		},
	}

	t.Run("no mismatch when bootstrap matches committed", func(t *testing.T) {
		local := map[string]bool{"feature-a": true}
		require.Empty(t, bootstrapMismatches(local, committed))
	})

	t.Run("value mismatch is reported", func(t *testing.T) {
		// local says feature-a=false but committed has true
		local := map[string]bool{"feature-a": false}
		mm := bootstrapMismatches(local, committed)
		require.Len(t, mm, 1)
		require.Equal(t, "feature-a", mm[0].name)
		require.False(t, mm[0].localEnabled)
		require.True(t, mm[0].committedEnabled)
		require.False(t, mm[0].absent)
	})

	t.Run("feature absent from committed policy is reported", func(t *testing.T) {
		local := map[string]bool{"future-feature": true}
		mm := bootstrapMismatches(local, committed)
		require.Len(t, mm, 1)
		require.Equal(t, "future-feature", mm[0].name)
		require.True(t, mm[0].absent)
	})

	t.Run("matching value produces no mismatch even if source differs", func(t *testing.T) {
		// feature-b is disabled by operator; local also says disabled — no mismatch
		local := map[string]bool{"feature-b": false}
		require.Empty(t, bootstrapMismatches(local, committed))
	})

	t.Run("empty local bootstrap produces no mismatches", func(t *testing.T) {
		require.Empty(t, bootstrapMismatches(nil, committed))
	})
}
