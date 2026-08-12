// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeatureGateSetRequest_RequestDatacenter(t *testing.T) {
	req := &FeatureGateSetRequest{Datacenter: "dc2"}
	require.Equal(t, "dc2", req.RequestDatacenter())
}

func TestFeatureGatePolicyClone(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var policy *FeatureGatePolicy
		require.Nil(t, policy.Clone())
	})

	t.Run("nil settings", func(t *testing.T) {
		policy := &FeatureGatePolicy{RaftIndex: RaftIndex{ModifyIndex: 7}}
		clone := policy.Clone()
		require.Equal(t, policy, clone)
		require.Nil(t, clone.Settings)
	})

	t.Run("deep copies settings", func(t *testing.T) {
		policy := &FeatureGatePolicy{
			Settings: map[string]FeatureGateSetting{
				"gate": {Enabled: true, Source: FeatureGateSourceOperator},
			},
			RaftIndex: RaftIndex{CreateIndex: 3, ModifyIndex: 7},
		}
		clone := policy.Clone()
		require.Equal(t, policy, clone)

		clone.Settings["gate"] = FeatureGateSetting{Enabled: false, Source: FeatureGateSourceBootstrap}
		require.True(t, policy.Settings["gate"].Enabled)
		require.Equal(t, FeatureGateSourceOperator, policy.Settings["gate"].Source)
	})
}

func TestFeatureGateStatusClone(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var status *FeatureGateStatus
		require.Nil(t, status.Clone())
	})

	t.Run("nil features", func(t *testing.T) {
		status := &FeatureGateStatus{PolicyIndex: 5, RegistryDigest: "digest"}
		clone := status.Clone()
		require.Equal(t, status, clone)
		require.Nil(t, clone.Features)
	})

	t.Run("deep copies features", func(t *testing.T) {
		status := &FeatureGateStatus{
			PolicyIndex:    5,
			RegistryDigest: "digest",
			Features: map[string]ResolvedFeatureGate{
				"gate": {
					DesiredEnabled:   true,
					EffectiveEnabled: true,
					Eligible:         true,
					Source:           string(FeatureGateSourceOperator),
					Reason:           FeatureGateReasonOperatorEnabled,
				},
			},
			RaftIndex: RaftIndex{CreateIndex: 8, ModifyIndex: 9},
		}
		clone := status.Clone()
		require.Equal(t, status, clone)

		clone.Features["gate"] = ResolvedFeatureGate{}
		require.True(t, status.Features["gate"].EffectiveEnabled)
		require.Equal(t, FeatureGateReasonOperatorEnabled, status.Features["gate"].Reason)
	})
}
