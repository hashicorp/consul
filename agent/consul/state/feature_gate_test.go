// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestStateStore_FeatureGateUpdate(t *testing.T) {
	store := testStateStore(t)

	policy := &structs.FeatureGatePolicy{Settings: map[string]structs.FeatureGateSetting{
		"test-feature": {Enabled: true, Source: structs.FeatureGateSourceBootstrap},
	}}
	status := &structs.FeatureGateStatus{
		RegistryDigest: "digest-1",
		Features: map[string]structs.ResolvedFeatureGate{
			"test-feature": {DesiredEnabled: true, EffectiveEnabled: true, Eligible: true},
		},
	}

	applied, err := store.FeatureGateUpdate(10, &structs.FeatureGateUpdateRequest{Policy: policy, Status: status})
	require.NoError(t, err)
	require.True(t, applied)

	index, storedPolicy, storedStatus, err := store.FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.Equal(t, uint64(10), index)
	require.Equal(t, uint64(10), storedPolicy.CreateIndex)
	require.Equal(t, uint64(10), storedPolicy.ModifyIndex)
	require.Equal(t, uint64(10), storedStatus.PolicyIndex)
	require.Equal(t, uint64(10), storedStatus.CreateIndex)
	require.Equal(t, uint64(10), storedStatus.ModifyIndex)

	// Returned maps are defensive copies and cannot mutate memdb state.
	storedPolicy.Settings["test-feature"] = structs.FeatureGateSetting{}
	_, storedPolicy, _, err = store.FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.True(t, storedPolicy.Settings["test-feature"].Enabled)

	// A stale status CAS is a complete no-op.
	applied, err = store.FeatureGateUpdate(11, &structs.FeatureGateUpdateRequest{
		Status:              &structs.FeatureGateStatus{RegistryDigest: "stale"},
		ExpectedPolicyIndex: 10,
		ExpectedStatusIndex: 9,
	})
	require.NoError(t, err)
	require.False(t, applied)
	_, _, storedStatus, err = store.FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.Equal(t, "digest-1", storedStatus.RegistryDigest)

	// Leader reconciliation updates status only and preserves the policy index.
	reconciled := storedStatus.Clone()
	reconciled.RegistryDigest = "digest-2"
	applied, err = store.FeatureGateUpdate(11, &structs.FeatureGateUpdateRequest{
		Status:              reconciled,
		ExpectedPolicyIndex: 10,
		ExpectedStatusIndex: 10,
	})
	require.NoError(t, err)
	require.True(t, applied)
	_, storedPolicy, storedStatus, err = store.FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.Equal(t, uint64(10), storedPolicy.ModifyIndex)
	require.Equal(t, uint64(10), storedStatus.PolicyIndex)
	require.Equal(t, uint64(11), storedStatus.ModifyIndex)
}

func TestStateStore_FeatureGateUpdateRequiresPolicy(t *testing.T) {
	store := testStateStore(t)
	applied, err := store.FeatureGateUpdate(1, &structs.FeatureGateUpdateRequest{
		Status: &structs.FeatureGateStatus{},
	})
	require.False(t, applied)
	require.ErrorContains(t, err, "status cannot exist without policy")
}

func TestStateStore_FeatureGatesSnapshotRestore(t *testing.T) {
	store := testStateStore(t)
	applied, err := store.FeatureGateUpdate(42, &structs.FeatureGateUpdateRequest{
		Policy: &structs.FeatureGatePolicy{Settings: map[string]structs.FeatureGateSetting{
			"test-feature": {Enabled: true, Source: structs.FeatureGateSourceOperator},
		}},
		Status: &structs.FeatureGateStatus{
			PolicyIndex:    42,
			RegistryDigest: "digest",
			Features: map[string]structs.ResolvedFeatureGate{
				"test-feature": {DesiredEnabled: true, EffectiveEnabled: true, Eligible: true},
			},
		},
	})
	require.NoError(t, err)
	require.True(t, applied)

	snapshot := store.Snapshot()
	defer snapshot.Close()
	featureGates, err := snapshot.FeatureGates()
	require.NoError(t, err)
	require.NotNil(t, featureGates)

	restoredStore := testStateStore(t)
	restore := restoredStore.Restore()
	require.NoError(t, restore.FeatureGates(featureGates))
	restore.Commit()

	index, policy, status, err := restoredStore.FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.Equal(t, uint64(42), index)
	require.Equal(t, featureGates.Policy, policy)
	require.Equal(t, featureGates.Status, status)
}
