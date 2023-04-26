// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateEnterpriseConfigKeys(t *testing.T) {
	// ensure that all the enterprise configurations
	type testCase struct {
		config  Config
		badKeys []string
		check   func(t *testing.T, c *Config)
	}

	boolVal := true
	stringVal := "string"

	cases := map[string]testCase{
		"read_replica": {
			config: Config{
				ReadReplica: &boolVal,
			},
			badKeys: []string{"read_replica (or the deprecated non_voting_server)"},
		},
		"segment": {
			config: Config{
				SegmentName: &stringVal,
			},
			badKeys: []string{"segment"},
		},
		"segments": {
			config: Config{
				Segments: []Segment{{Name: &stringVal}},
			},
			badKeys: []string{"segments"},
		},
		"autopilot.redundancy_zone_tag": {
			config: Config{
				Autopilot: Autopilot{
					RedundancyZoneTag: &stringVal,
				},
			},
			badKeys: []string{"autopilot.redundancy_zone_tag"},
		},
		"autopilot.upgrade_version_tag": {
			config: Config{
				Autopilot: Autopilot{
					UpgradeVersionTag: &stringVal,
				},
			},
			badKeys: []string{"autopilot.upgrade_version_tag"},
		},
		"autopilot.disable_upgrade_migration": {
			config: Config{
				Autopilot: Autopilot{DisableUpgradeMigration: &boolVal},
			},
			badKeys: []string{"autopilot.disable_upgrade_migration"},
		},
		"dns_config.prefer_namespace": {
			config: Config{
				DNS: DNS{PreferNamespace: &boolVal},
			},
			badKeys: []string{"dns_config.prefer_namespace"},
			check: func(t *testing.T, c *Config) {
				require.Nil(t, c.DNS.PreferNamespace)
			},
		},
		"acl.msp_disable_bootstrap": {
			config: Config{
				ACL: ACL{MSPDisableBootstrap: &boolVal},
			},
			badKeys: []string{"acl.msp_disable_bootstrap"},
			check: func(t *testing.T, c *Config) {
				require.Nil(t, c.ACL.MSPDisableBootstrap)
			},
		},
		"acl.tokens.managed_service_provider": {
			config: Config{
				ACL: ACL{
					Tokens: Tokens{
						ManagedServiceProvider: []ServiceProviderToken{
							{
								AccessorID: &stringVal,
								SecretID:   &stringVal,
							},
						},
					},
				},
			},
			badKeys: []string{"acl.tokens.managed_service_provider"},
			check: func(t *testing.T, c *Config) {
				require.Empty(t, c.ACL.Tokens.ManagedServiceProvider)
				require.Nil(t, c.ACL.Tokens.ManagedServiceProvider)
			},
		},
		"license_path": {
			config: Config{
				LicensePath: &stringVal,
			},
			badKeys: []string{"license_path"},
			check: func(t *testing.T, c *Config) {
				require.Empty(t, c.LicensePath)
			},
		},
		"reporting.license.enabled": {
			config: Config{
				Reporting: Reporting{
					License: License{
						Enabled: &boolVal,
					},
				},
			},
			badKeys: []string{"reporting.license.enabled"},
			check: func(t *testing.T, c *Config) {
				require.Nil(t, c.Reporting.License.Enabled)
			},
		},
		"multi": {
			config: Config{
				ReadReplica: &boolVal,
				SegmentName: &stringVal,
				ACL: ACL{
					Tokens: Tokens{
						DeprecatedTokens: DeprecatedTokens{AgentMaster: &stringVal},
					},
				},
			},
			badKeys: []string{"read_replica (or the deprecated non_voting_server)", "segment"},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validateEnterpriseConfigKeys(&tcase.config)
			if len(tcase.badKeys) == 0 {
				require.Len(t, errs, 0)
				return
			}

			var expected []error
			for _, k := range tcase.badKeys {
				expected = append(expected, enterpriseConfigKeyError{key: k})
			}
			require.ElementsMatch(t, expected, errs)

			if tcase.check != nil {
				tcase.check(t, &tcase.config)
			}
		})
	}
}
