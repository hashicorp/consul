// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package featuregate

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-version"
)

func TestResolve(t *testing.T) {
	definition := Definition{
		Name:                 "test-feature",
		Stage:                StageExperimental,
		MinVersion:           version.Must(version.NewVersion("2.1.0")),
		BeforeMinimumVersion: BeforeMinimumVersionDisabled,
		Description:          "test",
		Owner:                "test",
	}

	tests := map[string]struct {
		defaultEnabled  bool
		setting         *Setting
		meetsMinVersion bool
		membersFound    bool
		expected        Resolution
	}{
		"default disabled, eligible": {
			meetsMinVersion: true,
			membersFound:    true,
			expected:        Resolution{Eligible: true, Source: SourceRegistryDefault, Reason: ReasonDefaultDisabled},
		},
		"default enabled, eligible": {
			defaultEnabled:  true,
			meetsMinVersion: true,
			membersFound:    true,
			expected:        Resolution{DesiredEnabled: true, EffectiveEnabled: true, Eligible: true, Source: SourceRegistryDefault, Reason: ReasonDefaultEnabled},
		},
		"operator enabled, eligible": {
			setting:         &Setting{Enabled: true, Source: SourceOperator},
			meetsMinVersion: true,
			membersFound:    true,
			expected:        Resolution{DesiredEnabled: true, EffectiveEnabled: true, Eligible: true, Source: SourceOperator, Reason: ReasonOperatorEnabled},
		},
		"operator disabled, eligible": {
			setting:         &Setting{Source: SourceOperator},
			meetsMinVersion: true,
			membersFound:    true,
			expected:        Resolution{Source: SourceOperator, Eligible: true, Reason: ReasonOperatorDisabled},
		},
		"bootstrap enabled, eligible": {
			setting:         &Setting{Enabled: true, Source: SourceBootstrap},
			meetsMinVersion: true,
			membersFound:    true,
			expected:        Resolution{DesiredEnabled: true, EffectiveEnabled: true, Eligible: true, Source: SourceBootstrap, Reason: ReasonBootstrapEnabled},
		},
		"bootstrap disabled over enabled default": {
			defaultEnabled:  true,
			setting:         &Setting{Source: SourceBootstrap},
			meetsMinVersion: true,
			membersFound:    true,
			expected:        Resolution{Eligible: true, Source: SourceBootstrap, Reason: ReasonBootstrapDisabled},
		},
		"operator enabled but below minimum version": {
			setting:         &Setting{Enabled: true, Source: SourceOperator},
			meetsMinVersion: false,
			membersFound:    true,
			expected:        Resolution{DesiredEnabled: true, Source: SourceOperator, Reason: ReasonBelowMinimumVersion},
		},
		"operator enabled but no servers found": {
			setting:         &Setting{Enabled: true, Source: SourceOperator},
			meetsMinVersion: false,
			membersFound:    false,
			expected:        Resolution{DesiredEnabled: true, Source: SourceOperator, Reason: ReasonNoServerVersionData},
		},
		"default enabled but no servers found": {
			defaultEnabled:  true,
			meetsMinVersion: false,
			membersFound:    false,
			expected:        Resolution{DesiredEnabled: true, Source: SourceRegistryDefault, Reason: ReasonNoServerVersionData},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			definition.DefaultEnabled = tc.defaultEnabled
			require.Equal(t, tc.expected, Resolve(definition, tc.setting, tc.meetsMinVersion, tc.membersFound))
		})
	}
}
