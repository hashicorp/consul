// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package fsm

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestFSM_FeatureGateSnapshotRestore(t *testing.T) {
	newFSM := func() *FSM {
		return NewFromDeps(Deps{
			Logger: testutil.Logger(t),
			NewStateStore: func() *state.Store {
				return state.NewStateStore(nil)
			},
			StorageBackend: newStorageBackend(t, nil),
		})
	}

	original := newFSM()
	applied, err := original.state.FeatureGateUpdate(15, &structs.FeatureGateUpdateRequest{
		Policy: &structs.FeatureGatePolicy{Settings: map[string]structs.FeatureGateSetting{
			"test-feature": {Enabled: true, Source: structs.FeatureGateSourceOperator},
		}},
		Status: &structs.FeatureGateStatus{
			RegistryDigest: "digest",
			Features: map[string]structs.ResolvedFeatureGate{
				"test-feature": {DesiredEnabled: true, EffectiveEnabled: true, Eligible: true},
			},
		},
	})
	require.NoError(t, err)
	require.True(t, applied)
	_, expectedPolicy, expectedStatus, err := original.state.FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)

	snapshot, err := original.Snapshot()
	require.NoError(t, err)
	defer snapshot.Release()
	sink := &MockSink{Buffer: bytes.NewBuffer(nil)}
	require.NoError(t, snapshot.Persist(sink))

	restored := newFSM()
	require.NoError(t, restored.Restore(sink))
	_, actualPolicy, actualStatus, err := restored.state.FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.Equal(t, expectedPolicy, actualPolicy)
	require.Equal(t, expectedStatus, actualStatus)
}
