// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

// leader_feature_gate_reconcile_test.go covers reconcileFeatureGates,
// runFeatureGateCache, and related helpers that were not covered by the
// existing leader_feature_gate_test.go.

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"
	goversion "github.com/hashicorp/go-version"

	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
	raftstorage "github.com/hashicorp/consul/internal/storage/raft"
	"github.com/hashicorp/consul/testrpc"
)

// ---------------------------------------------------------------------------
// resolveFeatureGateStatus – nil policy (used before first bootstrap)
// ---------------------------------------------------------------------------

func TestResolveFeatureGateStatus_NilPolicy(t *testing.T) {
	status := resolveFeatureGateStatus(featuregate.DefaultRegistry(), nil, func(_ *goversion.Version) (bool, bool) {
		return true, true
	})
	require.NotNil(t, status)
	require.Equal(t, uint64(0), status.PolicyIndex)
	require.NotEmpty(t, status.Features)

	resolved := status.Features[featuregate.APIGatewayUpstreamRouting.String()]
	// With nil policy the registry default applies (DefaultEnabled=false).
	require.False(t, resolved.DesiredEnabled)
	require.Equal(t, string(featuregate.SourceRegistryDefault), resolved.Source)
}

// ---------------------------------------------------------------------------
// featureGateStatusesEqual edge-cases
// ---------------------------------------------------------------------------

func TestFeatureGateStatusesEqual_NilBothAreNotEqual(t *testing.T) {
	require.False(t, featureGateStatusesEqual(nil, nil))
}

func TestFeatureGateStatusesEqual_LeftNil(t *testing.T) {
	right := &structs.FeatureGateStatus{
		PolicyIndex: 1, RegistryDigest: "d",
		Features: map[string]structs.ResolvedFeatureGate{},
	}
	require.False(t, featureGateStatusesEqual(nil, right))
}

func TestFeatureGateStatusesEqual_RightNil(t *testing.T) {
	left := &structs.FeatureGateStatus{
		PolicyIndex: 1, RegistryDigest: "d",
		Features: map[string]structs.ResolvedFeatureGate{},
	}
	require.False(t, featureGateStatusesEqual(left, nil))
}

func TestFeatureGateStatusesEqual_DifferentPolicyIndex(t *testing.T) {
	left := &structs.FeatureGateStatus{PolicyIndex: 1, RegistryDigest: "d", Features: map[string]structs.ResolvedFeatureGate{}}
	right := &structs.FeatureGateStatus{PolicyIndex: 2, RegistryDigest: "d", Features: map[string]structs.ResolvedFeatureGate{}}
	require.False(t, featureGateStatusesEqual(left, right))
}

func TestFeatureGateStatusesEqual_DifferentDigest(t *testing.T) {
	left := &structs.FeatureGateStatus{PolicyIndex: 1, RegistryDigest: "a", Features: map[string]structs.ResolvedFeatureGate{}}
	right := &structs.FeatureGateStatus{PolicyIndex: 1, RegistryDigest: "b", Features: map[string]structs.ResolvedFeatureGate{}}
	require.False(t, featureGateStatusesEqual(left, right))
}

// ---------------------------------------------------------------------------
// runFeatureGateCache – reset on FSM abandon, error retry
// ---------------------------------------------------------------------------

// TestRunFeatureGateCache_PublishesInitialState verifies that runFeatureGateCache
// publishes the committed FSM state when a policy/status pair already exists.
func TestRunFeatureGateCache_PublishesInitialState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	store := state.NewStateStore(nil)
	// Seed a committed status directly into the state store.
	req := &structs.FeatureGateUpdateRequest{
		Policy: &structs.FeatureGatePolicy{
			Settings: map[string]structs.FeatureGateSetting{
				featuregate.APIGatewayUpstreamRouting.String(): {Enabled: true, Source: structs.FeatureGateSourceBootstrap},
			},
		},
		Status: &structs.FeatureGateStatus{
			Features: map[string]structs.ResolvedFeatureGate{
				featuregate.APIGatewayUpstreamRouting.String(): {
					EffectiveEnabled: true,
					DesiredEnabled:   true,
					Eligible:         true,
					Source:           string(structs.FeatureGateSourceBootstrap),
					Reason:           structs.FeatureGateReasonBootstrapEnabled,
				},
			},
		},
	}
	ok, err := store.FeatureGateUpdate(10, req)
	require.NoError(t, err)
	require.True(t, ok)

	fsmServer := fsm.NewFromDeps(fsm.Deps{
		Logger:         hclog.NewNullLogger(),
		NewStateStore:  func() *state.Store { return store },
		StorageBackend: fsm.NullStorageBackend,
	})

	gateStore := &featuregate.Store{}
	nullLogger := hclog.NewInterceptLogger(&hclog.LoggerOptions{Output: io.Discard})
	server := &Server{
		logger:           nullLogger,
		featureGateStore: gateStore,
		fsm:              fsmServer,
	}

	go server.runFeatureGateCache(ctx)
	// Give the goroutine a moment to read from the state store and publish.
	require.Eventually(t, func() bool {
		return gateStore.Enabled(featuregate.APIGatewayUpstreamRouting)
	}, 300*time.Millisecond, 10*time.Millisecond, "runFeatureGateCache should publish enabled=true")
}

// TestRunFeatureGateCache_RapidChangesDoNotLag verifies that a burst of gate
// changes is never held back by a loop rate limit.
func TestRunFeatureGateCache_RapidChangesDoNotLag(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	name := featuregate.APIGatewayUpstreamRouting.String()
	store := state.NewStateStore(nil)
	fsmServer := fsm.NewFromDeps(fsm.Deps{
		Logger:         hclog.NewNullLogger(),
		NewStateStore:  func() *state.Store { return store },
		StorageBackend: fsm.NullStorageBackend,
	})
	gateStore := &featuregate.Store{}
	server := &Server{
		logger:           hclog.NewInterceptLogger(&hclog.LoggerOptions{Output: io.Discard}),
		featureGateStore: gateStore,
		fsm:              fsmServer,
	}
	go server.runFeatureGateCache(ctx)

	var previous uint64
	for i := 1; i <= 12; i++ {
		want := i%2 == 1
		idx := uint64(10 + i)
		ok, err := store.FeatureGateUpdate(idx, &structs.FeatureGateUpdateRequest{
			Policy: &structs.FeatureGatePolicy{Settings: map[string]structs.FeatureGateSetting{
				name: {Enabled: want, Source: structs.FeatureGateSourceOperator},
			}},
			Status: &structs.FeatureGateStatus{Features: map[string]structs.ResolvedFeatureGate{
				name: {DesiredEnabled: want, EffectiveEnabled: want, Eligible: true},
			}},
			ExpectedPolicyIndex: previous,
			ExpectedStatusIndex: previous,
		})
		require.NoError(t, err)
		require.True(t, ok)
		previous = idx

		require.Eventually(t, func() bool {
			return gateStore.Enabled(featuregate.APIGatewayUpstreamRouting) == want
		}, time.Second, 5*time.Millisecond, "change %d should reach the local store promptly", i)
	}
}

// TestRunFeatureGateCache_ResetOnNoStatus verifies that when the FSM has no
// committed status, the store is reset (fail-closed).
func TestRunFeatureGateCache_ResetOnNoStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	store := state.NewStateStore(nil)
	fsmServer := fsm.NewFromDeps(fsm.Deps{
		Logger:         hclog.NewNullLogger(),
		NewStateStore:  func() *state.Store { return store },
		StorageBackend: fsm.NullStorageBackend,
	})

	nullLogger := hclog.NewInterceptLogger(&hclog.LoggerOptions{Output: io.Discard})
	gateStore := &featuregate.Store{}
	// Pre-seed a snapshot so we can confirm it gets cleared.
	gateStore.Publish(featuregate.Snapshot{
		StatusIndex: 5,
		Features:    map[string]bool{featuregate.APIGatewayUpstreamRouting.String(): true},
	})
	require.True(t, gateStore.Enabled(featuregate.APIGatewayUpstreamRouting))

	server := &Server{
		logger:           nullLogger,
		featureGateStore: gateStore,
		fsm:              fsmServer,
	}

	go server.runFeatureGateCache(ctx)
	<-ctx.Done()
	// After context expires (FSM still has no status), the store should be reset.
	require.Equal(t, featuregate.Snapshot{}, gateStore.Current())
}

// ---------------------------------------------------------------------------
// reconcileFeatureGates – initial bootstrap from config
// ---------------------------------------------------------------------------

func TestReconcileFeatureGates_InitialBootstrapFromConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	featureName := featuregate.APIGatewayUpstreamRouting.String()

	_, s := testServerWithConfig(t, func(c *Config) {
		c.FeatureGatesBootstrap = map[string]bool{featureName: true}
	})
	testrpc.WaitForLeader(t, s.RPC, "dc1")

	// Wait for the policy to reflect the bootstrap config.
	require.Eventually(t, func() bool {
		_, policy, status, _ := s.fsm.State().FeatureGatePolicyAndStatus(nil)
		if policy == nil || status == nil {
			return false
		}
		setting, ok := policy.Settings[featureName]
		return ok && setting.Enabled && setting.Source == structs.FeatureGateSourceBootstrap
	}, 5*time.Second, 50*time.Millisecond, "bootstrap config should be reflected in committed policy")

	frameworkVersion, err := s.GetSystemMetadata(structs.SystemMetadataFeatureGatesVersionKey)
	require.NoError(t, err)
	require.Equal(t, structs.SystemMetadataFeatureGatesVersionValue, frameworkVersion)
}

// ---------------------------------------------------------------------------
// reconcileFeatureGates – semantic no-op skips Raft write
// ---------------------------------------------------------------------------

func TestReconcileFeatureGates_SemanticNoOpSkipsRaft(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServer(t)
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	// Capture the current Raft last log index.
	_, _, status1, err := s.fsm.State().FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.NotNil(t, status1)
	modIdx1 := status1.ModifyIndex

	// Call reconcileFeatureGates again – status should not change because
	// nothing has changed.
	require.NoError(t, s.reconcileFeatureGates())

	_, _, status2, err := s.fsm.State().FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.NotNil(t, status2)

	require.Equal(t, modIdx1, status2.ModifyIndex,
		"reconcileFeatureGates should not commit a new Raft entry when nothing changed")
}

// With no alive or failed server in the datacenter (every server left or is
// leaving) the membership view is unknown and must not be committed as the
// cluster-wide state, or every gate is disabled until the next leader reconciles.
func TestReconcileFeatureGates_NoServerDataKeepsCommittedStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	featureName := featuregate.APIGatewayUpstreamRouting.String()
	_, s := testServerWithConfig(t, func(c *Config) {
		c.FeatureGatesBootstrap = map[string]bool{featureName: true}
	})
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	_, _, before, err := s.fsm.State().FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.True(t, before.Features[featureName].EffectiveEnabled)

	for _, srv := range s.serverLookup.Servers() {
		s.serverLookup.RemoveServer(srv)
	}
	require.NoError(t, s.reconcileFeatureGates())

	_, _, after, err := s.fsm.State().FeatureGatePolicyAndStatus(nil)
	require.NoError(t, err)
	require.Equal(t, before.ModifyIndex, after.ModifyIndex)
	require.True(t, after.Features[featureName].EffectiveEnabled)
}

func TestWarnBootstrapMismatchIfCommitted(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServerWithConfig(t, func(c *Config) {
		c.FeatureGatesBootstrap = map[string]bool{featuregate.APIGatewayUpstreamRouting.String(): true}
	})
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	waitForFeatureGateInit(t, s)

	require.True(t, s.warnBootstrapMismatchIfCommitted())
}

// ---------------------------------------------------------------------------
// bootstrapMismatches – additional edge cases (complement existing tests)
// ---------------------------------------------------------------------------

func TestBootstrapMismatches_MultipleEntries(t *testing.T) {
	committed := &structs.FeatureGatePolicy{
		Settings: map[string]structs.FeatureGateSetting{
			"alpha": {Enabled: true, Source: structs.FeatureGateSourceBootstrap},
			"beta":  {Enabled: false, Source: structs.FeatureGateSourceBootstrap},
		},
	}
	local := map[string]bool{
		"alpha": false, // mismatch
		"beta":  false, // matches
		"gamma": true,  // absent from committed
	}
	mm := bootstrapMismatches(local, committed)
	require.Len(t, mm, 2)

	names := make(map[string]bool, 2)
	for _, m := range mm {
		names[m.name] = true
	}
	require.True(t, names["alpha"], "alpha mismatch expected")
	require.True(t, names["gamma"], "gamma absent mismatch expected")
	require.False(t, names["beta"], "beta matches – should not be reported")
}

type bufferSnapshotSink struct{ bytes.Buffer }

func (*bufferSnapshotSink) ID() string    { return "buffer" }
func (*bufferSnapshotSink) Cancel() error { return nil }
func (*bufferSnapshotSink) Close() error  { return nil }

func newTestRaftStorageBackend(t *testing.T) *raftstorage.Backend {
	t.Helper()
	backend, err := raftstorage.NewBackend(nil, hclog.NewNullLogger())
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go backend.Run(ctx)
	return backend
}

// TestRunFeatureGateCache_RestoreOlderSnapshot verifies that restoring a
// snapshot whose feature-gate status is older than the cached generation
// installs the restored decision instead of keeping the pre-restore one, and
// that the change is visible to blocking queries as a newer index.
func TestRunFeatureGateCache_RestoreOlderSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeded := func(index uint64, enabled bool) *state.Store {
		store := state.NewStateStore(nil)
		ok, err := store.FeatureGateUpdate(index, &structs.FeatureGateUpdateRequest{
			Policy: &structs.FeatureGatePolicy{Settings: map[string]structs.FeatureGateSetting{
				featuregate.LocalizedDNS.String(): {Enabled: enabled, Source: structs.FeatureGateSourceOperator},
			}},
			Status: &structs.FeatureGateStatus{Features: map[string]structs.ResolvedFeatureGate{
				featuregate.LocalizedDNS.String(): {EffectiveEnabled: enabled, DesiredEnabled: enabled, Eligible: true},
			}},
		})
		require.NoError(t, err)
		require.True(t, ok)
		return store
	}

	// Capture a snapshot with an older status that disables localized-dns.
	older := seeded(10, false)
	olderFSM := fsm.NewFromDeps(fsm.Deps{
		Logger:         hclog.NewNullLogger(),
		NewStateStore:  func() *state.Store { return older },
		StorageBackend: newTestRaftStorageBackend(t),
	})
	snap, err := olderFSM.Snapshot()
	require.NoError(t, err)
	sink := &bufferSnapshotSink{}
	require.NoError(t, snap.Persist(sink))

	live := seeded(50, true)
	fsmServer := fsm.NewFromDeps(fsm.Deps{
		Logger: hclog.NewNullLogger(),
		NewStateStore: func() *state.Store {
			if live != nil {
				store := live
				live = nil
				return store
			}
			return state.NewStateStore(nil)
		},
		StorageBackend: newTestRaftStorageBackend(t),
	})
	gateStore := &featuregate.Store{}
	server := &Server{
		logger:           hclog.NewInterceptLogger(&hclog.LoggerOptions{Output: io.Discard}),
		featureGateStore: gateStore,
		fsm:              fsmServer,
	}
	go server.runFeatureGateCache(ctx)

	require.Eventually(t, func() bool {
		return gateStore.Enabled(featuregate.LocalizedDNS)
	}, 2*time.Second, 10*time.Millisecond, "live status must be published")
	_, beforeIndex := gateStore.ObserveForQuery(nil, featuregate.LocalizedDNS, 0)

	require.NoError(t, fsmServer.Restore(io.NopCloser(&sink.Buffer)))

	require.Eventually(t, func() bool {
		return !gateStore.Enabled(featuregate.LocalizedDNS)
	}, 2*time.Second, 10*time.Millisecond, "restored (older) status must replace the pre-restore decision")
	require.Equal(t, uint64(10), gateStore.Current().StatusIndex)
	_, afterIndex := gateStore.ObserveForQuery(nil, featuregate.LocalizedDNS, 0)
	require.Greater(t, afterIndex, beforeIndex, "the restore must be visible to blocking queries")
}
