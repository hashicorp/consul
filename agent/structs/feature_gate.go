// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

type FeatureGateSettingSource string

const (
	FeatureGateSourceBootstrap FeatureGateSettingSource = "bootstrap"
	FeatureGateSourceOperator  FeatureGateSettingSource = "operator"
)

type FeatureGateReason string

const (
	// Source-and-value reasons: emitted when the cluster meets the minimum version.
	FeatureGateReasonDefaultEnabled    FeatureGateReason = "default-enabled"
	FeatureGateReasonDefaultDisabled   FeatureGateReason = "default-disabled"
	FeatureGateReasonBootstrapEnabled  FeatureGateReason = "bootstrap-enabled"
	FeatureGateReasonBootstrapDisabled FeatureGateReason = "bootstrap-disabled"
	FeatureGateReasonOperatorEnabled   FeatureGateReason = "operator-enabled"
	FeatureGateReasonOperatorDisabled  FeatureGateReason = "operator-disabled"
	// Eligibility reasons: emitted when the cluster does not yet qualify.
	FeatureGateReasonBelowMinimumVersion FeatureGateReason = "below-minimum-version"
	FeatureGateReasonNoServerVersionData FeatureGateReason = "no-server-version-data"
	// FeatureGateReasonUnknownFeature is used when a policy setting references a
	// feature name that is absent from this binary's registry. The intent is
	// preserved in policy; the effective value is fail-closed until a leader with
	// the matching registry wins and re-resolves the setting.
	FeatureGateReasonUnknownFeature FeatureGateReason = "unknown-feature"

	// Deprecated aliases kept for callers predating the granular reason vocabulary.
	FeatureGateReasonEnabled          = FeatureGateReasonDefaultEnabled
	FeatureGateReasonDisabledByPolicy = FeatureGateReasonDefaultDisabled
)

type FeatureGateSetting struct {
	Enabled bool
	Source  FeatureGateSettingSource
}

// FeatureGatePolicy stores mutable operator/bootstrap intent. A missing entry
// means the compiled registry default applies.
type FeatureGatePolicy struct {
	Settings map[string]FeatureGateSetting
	RaftIndex
}

type ResolvedFeatureGate struct {
	DesiredEnabled   bool
	EffectiveEnabled bool
	Eligible         bool
	Source           string
	Reason           FeatureGateReason
}

// FeatureGateStatus is the final cluster-wide materialized decision consumed
// by runtime caches. It is committed through Raft with the policy generation
// from which it was resolved.
type FeatureGateStatus struct {
	PolicyIndex    uint64
	RegistryDigest string
	Features       map[string]ResolvedFeatureGate
	RaftIndex
}

// FeatureGateUpdateRequest atomically updates status and, when Policy is
// non-nil, policy. Expected indexes fence stale leader reconciliation and
// concurrent operator writes. Index zero matches an absent singleton.
type FeatureGateUpdateRequest struct {
	Policy              *FeatureGatePolicy
	Status              *FeatureGateStatus
	ExpectedPolicyIndex uint64
	ExpectedStatusIndex uint64
}

// FeatureGateSnapshot is encoded as one snapshot record so policy and its
// resolved status restore together.
type FeatureGateSnapshot struct {
	Policy *FeatureGatePolicy
	Status *FeatureGateStatus
}

type FeatureGateInfo struct {
	Name                 string
	Stage                string
	MinVersion           string
	DefaultEnabled       bool
	BeforeMinimumVersion string
	Description          string
	Owner                string
	DesiredEnabled       bool
	EffectiveEnabled     bool
	Eligible             bool
	Source               string
	Reason               FeatureGateReason
	PolicyIndex          uint64
	StatusIndex          uint64
}

type FeatureGateQueryRequest struct {
	Name string
	DCSpecificRequest
}

type FeatureGateQueryResponse struct {
	Features []FeatureGateInfo
	// Uninitialized is true when the leader has not yet committed the first
	// feature-gate policy/status generation.  Callers should retry after a
	// short delay; blocking queries will return once the state is committed.
	Uninitialized bool
	QueryMeta
}

type FeatureGateSetRequest struct {
	Datacenter          string
	Name                string
	Enabled             bool
	ExpectedPolicyIndex uint64
	WriteRequest
}

func (r *FeatureGateSetRequest) RequestDatacenter() string { return r.Datacenter }

type FeatureGateSetResponse struct {
	Applied bool
	Feature FeatureGateInfo
}

func (p *FeatureGatePolicy) Clone() *FeatureGatePolicy {
	if p == nil {
		return nil
	}
	clone := *p
	if p.Settings != nil {
		clone.Settings = make(map[string]FeatureGateSetting, len(p.Settings))
		for name, setting := range p.Settings {
			clone.Settings[name] = setting
		}
	}
	return &clone
}

func (s *FeatureGateStatus) Clone() *FeatureGateStatus {
	if s == nil {
		return nil
	}
	clone := *s
	if s.Features != nil {
		clone.Features = make(map[string]ResolvedFeatureGate, len(s.Features))
		for name, feature := range s.Features {
			clone.Features[name] = feature
		}
	}
	return &clone
}
