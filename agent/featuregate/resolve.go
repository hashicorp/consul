// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package featuregate

type Source string

const (
	SourceRegistryDefault Source = "registry-default"
	SourceBootstrap       Source = "bootstrap"
	SourceOperator        Source = "operator"
)

type Reason string

const (
	// Source-and-value reasons: emitted when the cluster meets the minimum version.
	ReasonDefaultEnabled    Reason = "default-enabled"
	ReasonDefaultDisabled   Reason = "default-disabled"
	ReasonBootstrapEnabled  Reason = "bootstrap-enabled"
	ReasonBootstrapDisabled Reason = "bootstrap-disabled"
	ReasonOperatorEnabled   Reason = "operator-enabled"
	ReasonOperatorDisabled  Reason = "operator-disabled"
	// Eligibility reasons: emitted when the cluster does not yet qualify.
	ReasonBelowMinimumVersion  Reason = "below-minimum-version"
	ReasonNoServerVersionData  Reason = "no-server-version-data"

	// Deprecated aliases kept so existing callers that compare against the old
	// values continue to compile. New code should use the named constants above.
	ReasonEnabled          = ReasonDefaultEnabled
	ReasonDisabledByPolicy = ReasonDefaultDisabled
)

// Setting is optional mutable intent. A missing Setting selects the registry
// default.
type Setting struct {
	Enabled bool
	Source  Source
}

// Resolution is the complete cluster-level decision for one definition.
type Resolution struct {
	DesiredEnabled   bool
	EffectiveEnabled bool
	Eligible         bool
	Source           Source
	Reason           Reason
}

// Resolve applies operator/bootstrap intent, the registry default, and the
// minimum-version result. It is deterministic and has no runtime dependencies.
//
// meetsMinimumVersion reports whether every alive/failed server is at or above
// definition.MinVersion. membersFound reports whether Serf returned at least
// one server; when false the cluster state is unknown and the feature is
// fail-closed with ReasonNoServerVersionData regardless of intent.
func Resolve(definition Definition, setting *Setting, meetsMinimumVersion, membersFound bool) Resolution {
	resolution := Resolution{
		DesiredEnabled: definition.DefaultEnabled,
		Eligible:       meetsMinimumVersion && membersFound,
		Source:         SourceRegistryDefault,
	}
	if setting != nil {
		resolution.DesiredEnabled = setting.Enabled
		resolution.Source = setting.Source
	}

	if !membersFound {
		resolution.EffectiveEnabled = false
		resolution.Reason = ReasonNoServerVersionData
		return resolution
	}
	if !meetsMinimumVersion {
		resolution.EffectiveEnabled = false
		resolution.Reason = ReasonBelowMinimumVersion
		return resolution
	}

	resolution.EffectiveEnabled = resolution.DesiredEnabled
	resolution.Reason = eligibleReason(resolution.Source, resolution.EffectiveEnabled)
	return resolution
}

// eligibleReason maps source+enabled to the appropriate named reason constant.
func eligibleReason(source Source, enabled bool) Reason {
	switch source {
	case SourceBootstrap:
		if enabled {
			return ReasonBootstrapEnabled
		}
		return ReasonBootstrapDisabled
	case SourceOperator:
		if enabled {
			return ReasonOperatorEnabled
		}
		return ReasonOperatorDisabled
	default: // SourceRegistryDefault
		if enabled {
			return ReasonDefaultEnabled
		}
		return ReasonDefaultDisabled
	}
}
