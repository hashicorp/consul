// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	featureGateReconciliationRoutineName = "feature-gate reconciliation"
	featureGateReconciliationInterval    = 10 * time.Second
)

func (s *Server) startFeatureGateReconciliation(ctx context.Context) {
	s.leaderRoutineManager.Start(ctx, featureGateReconciliationRoutineName, s.runFeatureGateReconciliation)
}

func (s *Server) stopFeatureGateReconciliation() {
	s.leaderRoutineManager.Stop(featureGateReconciliationRoutineName)
}

func (s *Server) runFeatureGateReconciliation(ctx context.Context) error {
	ticker := time.NewTicker(featureGateReconciliationInterval)
	defer ticker.Stop()

	for {
		if err := s.reconcileFeatureGates(); err != nil {
			s.logger.Error("failed to reconcile feature gates", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Server) reconcileFeatureGates() error {
	frameworkReady, err := s.ensureFeatureGateFrameworkVersion()
	if err != nil {
		return err
	}
	if !frameworkReady {
		return nil
	}

	_, policy, currentStatus, err := s.fsm.State().FeatureGatePolicyAndStatus(nil)
	if err != nil {
		return err
	}

	request := structs.FeatureGateUpdateRequest{}
	if policy == nil {
		policy = &structs.FeatureGatePolicy{Settings: make(map[string]structs.FeatureGateSetting, len(s.config.FeatureGatesBootstrap))}
		for name, enabled := range s.config.FeatureGatesBootstrap {
			policy.Settings[name] = structs.FeatureGateSetting{
				Enabled: enabled,
				Source:  structs.FeatureGateSourceBootstrap,
			}
		}
		request.Policy = policy
	} else {
		// Policy already committed — warn if local bootstrap config diverges from
		// what was originally stored so operators notice configuration drift.
		// Never overwrite committed policy from local config.
		s.warnBootstrapMismatch(policy)
	}

	status := resolveFeatureGateStatus(s.featureGateRegistry, policy, func(minimum *version.Version) (bool, bool) {
		return ServersInDCMeetMinimumVersion(s, s.config.Datacenter, minimum)
	})
	if request.Policy == nil && featureGateStatusesEqual(currentStatus, status) {
		return nil
	}

	request.Status = status
	if policy.ModifyIndex != 0 {
		request.ExpectedPolicyIndex = policy.ModifyIndex
	}
	if currentStatus != nil {
		request.ExpectedStatusIndex = currentStatus.ModifyIndex
	}

	// Feature-gate state is safe for pre-framework servers to omit: those
	// binaries cannot consume it, and current binaries commit the complete
	// policy and resolved status together. Always mark the command ignorable so
	// a downgraded follower can replay entries written before it joined.
	response, err := s.leaderRaftApply(
		"FeatureGate.Apply",
		structs.FeatureGateRequestType|structs.IgnoreUnknownTypeFlag,
		request,
	)
	if err != nil {
		return err
	}
	applied, ok := response.(bool)
	if !ok {
		return fmt.Errorf("feature-gate update returned unexpected response %T", response)
	}
	if !applied {
		// A concurrent operator update or newer reconciler won the CAS. The
		// next iteration will read and resolve that committed generation.
		return nil
	}
	return nil
}

// ensureFeatureGateFrameworkVersion durably activates the feature-gate wire
// and storage schema before the leader emits its first FeatureGate Raft
// command. Once written, the SystemMetadata marker is authoritative and avoids
// re-deriving cluster capability from transient membership on every reconcile.
func (s *Server) ensureFeatureGateFrameworkVersion() (bool, error) {
	activated, err := s.GetSystemMetadata(structs.SystemMetadataFeatureGatesVersionKey)
	if err != nil {
		return false, fmt.Errorf("read feature-gate framework version: %w", err)
	}

	required := version.Must(version.NewVersion(structs.SystemMetadataFeatureGatesVersionValue))
	if activated != "" {
		active, err := version.NewVersion(activated)
		if err != nil {
			return false, fmt.Errorf("invalid feature-gate framework version %q in system metadata: %w", activated, err)
		}
		if active.GreaterThanOrEqual(required) {
			return true, nil
		}
	}

	meetsMinimum, serversFound := ServersInDCMeetMinimumVersion(s, s.config.Datacenter, required)
	if !meetsMinimum || !serversFound {
		// Older servers cannot decode the FeatureGate Raft command. Do not
		// create even an all-disabled policy until the framework floor is met.
		return false, nil
	}

	if err := s.SetSystemMetadataKey(structs.SystemMetadataFeatureGatesVersionKey, required.String()); err != nil {
		return false, fmt.Errorf("persist feature-gate framework version: %w", err)
	}
	return true, nil
}

type minimumVersionCheck func(*version.Version) (bool, bool)

func resolveFeatureGateStatus(registry featuregate.Registry, policy *structs.FeatureGatePolicy, check minimumVersionCheck) *structs.FeatureGateStatus {
	status := &structs.FeatureGateStatus{
		RegistryDigest: registry.Digest(),
		Features:       make(map[string]structs.ResolvedFeatureGate),
	}
	if policy != nil {
		status.PolicyIndex = policy.ModifyIndex
	}

	for _, definition := range registry.Definitions() {
		var setting *featuregate.Setting
		if policy != nil {
			if policySetting, ok := policy.Settings[definition.Name]; ok {
				setting = &featuregate.Setting{
					Enabled: policySetting.Enabled,
					Source:  featuregate.Source(policySetting.Source),
				}
			}
		}
		meetsMinimum, membersFound := check(definition.MinVersion)
		resolution := featuregate.Resolve(definition, setting, meetsMinimum, membersFound)
		status.Features[definition.Name] = structs.ResolvedFeatureGate{
			DesiredEnabled:   resolution.DesiredEnabled,
			EffectiveEnabled: resolution.EffectiveEnabled,
			Eligible:         resolution.Eligible,
			Source:           string(resolution.Source),
			Reason:           structs.FeatureGateReason(resolution.Reason),
		}
	}
	if policy != nil {
		for name, setting := range policy.Settings {
			if _, known := status.Features[name]; known {
				continue // already resolved above
			}
			// Unknown to this binary's registry — preserve intent, mark ineligible
			status.Features[name] = structs.ResolvedFeatureGate{
				DesiredEnabled:   setting.Enabled,
				EffectiveEnabled: false, // fail-closed
				Eligible:         false,
				Source:           string(setting.Source),
				Reason:           structs.FeatureGateReasonUnknownFeature, // new constant
			}
		}
	}
	return status
}

func featureGateStatusesEqual(current, candidate *structs.FeatureGateStatus) bool {
	// The Raft indexes are intentionally ignored here because the reconciler only
	// cares whether the resolved feature set is semantically unchanged.
	return current != nil && candidate != nil &&
		current.PolicyIndex == candidate.PolicyIndex &&
		current.RegistryDigest == candidate.RegistryDigest &&
		maps.Equal(current.Features, candidate.Features)
}

// runFeatureGateCache runs on every server. It reads only committed local FSM
// state and atomically publishes complete final generations.
func (s *Server) runFeatureGateCache(ctx context.Context) {
	retryLoopBackoff(ctx, func() error {
		stateStore := s.fsm.State()
		ws := memdb.NewWatchSet()
		ws.Add(stateStore.AbandonCh())
		_, _, status, err := stateStore.FeatureGatePolicyAndStatus(ws)
		if err != nil {
			s.logger.Error("failed to watch committed feature-gate status", "error", err)
			return err
		}
		if status != nil {
			features := make(map[string]bool, len(status.Features))
			for name, resolved := range status.Features {
				features[name] = resolved.EffectiveEnabled
			}
			published := s.featureGateStore.Publish(featuregate.Snapshot{
				StatusIndex:    status.ModifyIndex,
				PolicyIndex:    status.PolicyIndex,
				RegistryDigest: status.RegistryDigest,
				Features:       features,
			})
			if published {
				s.logger.Debug("feature-gate cache updated from committed FSM state",
					"status_index", status.ModifyIndex,
					"policy_index", status.PolicyIndex,
					"features", features,
				)
			}
		} else {
			// The current FSM state store has no committed feature-gate status yet
			// (new store after a snapshot restore, or a fresh pre-bootstrap cluster).
			// Reset to an uninitialized, fail-closed state so that a stale snapshot
			// from a previous generation cannot continue returning true for features
			// that have not been confirmed by the authoritative FSM.
			s.featureGateStore.Reset()
		}

		if err := ws.WatchCtx(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		return nil
	}, func(err error) {
		s.logger.Error("feature-gate cache watch failed, retrying", "error", err)
	})
}

// bootstrapMismatch describes one divergence between local bootstrap config and
// the already-committed Raft policy.
type bootstrapMismatch struct {
	name             string
	localEnabled     bool
	committedEnabled bool
	committedSource  structs.FeatureGateSettingSource
	absent           bool // true when the feature is not in the committed policy at all
}

// bootstrapMismatches returns one entry for each feature in localBootstrap whose
// value or presence diverges from the committed policy.  The return value is
// used by warnBootstrapMismatch and directly tested.
func bootstrapMismatches(localBootstrap map[string]bool, policy *structs.FeatureGatePolicy) []bootstrapMismatch {
	var out []bootstrapMismatch
	for name, localEnabled := range localBootstrap {
		committed, ok := policy.Settings[name]
		if !ok {
			out = append(out, bootstrapMismatch{name: name, localEnabled: localEnabled, absent: true})
			continue
		}
		if committed.Enabled != localEnabled {
			out = append(out, bootstrapMismatch{
				name:             name,
				localEnabled:     localEnabled,
				committedEnabled: committed.Enabled,
				committedSource:  committed.Source,
			})
		}
	}
	return out
}

// warnBootstrapMismatch logs a warning for each feature in the local
// feature_gates.bootstrap config whose value differs from the already-committed
// Raft policy.  This is purely diagnostic: local configuration never overrides
// committed policy.
func (s *Server) warnBootstrapMismatch(policy *structs.FeatureGatePolicy) {
	for _, m := range bootstrapMismatches(s.config.FeatureGatesBootstrap, policy) {
		if m.absent {
			s.logger.Warn("feature_gates.bootstrap contains a feature not present in committed policy; local config ignored",
				"feature", m.name,
				"local_enabled", m.localEnabled,
			)
		} else {
			s.logger.Warn("feature_gates.bootstrap diverges from committed Raft policy; local config ignored",
				"feature", m.name,
				"local_enabled", m.localEnabled,
				"committed_enabled", m.committedEnabled,
				"committed_source", m.committedSource,
			)
		}
	}
}
