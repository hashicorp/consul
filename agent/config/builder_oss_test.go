// +build !consulent

package config

import (
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
)

func TestBuilder_validateEnterpriseConfigKeys(t *testing.T) {
	// ensure that all the enterprise configurations
	type testCase struct {
		config  Config
		keys    []string
		badKeys []string
		check   func(t *testing.T, c *Config)
	}

	boolVal := true
	stringVal := "string"

	cases := map[string]testCase{
		"non_voting_server": {
			config: Config{
				NonVotingServer: &boolVal,
			},
			keys:    []string{"non_voting_server"},
			badKeys: []string{"non_voting_server"},
		},
		"segment": {
			config: Config{
				SegmentName: &stringVal,
			},
			keys:    []string{"segment"},
			badKeys: []string{"segment"},
		},
		"segments": {
			config: Config{
				Segments: []Segment{
					{Name: &stringVal},
				},
			},
			keys:    []string{"segments"},
			badKeys: []string{"segments"},
		},
		"autopilot.redundancy_zone_tag": {
			config: Config{
				Autopilot: Autopilot{
					RedundancyZoneTag: &stringVal,
				},
			},
			keys:    []string{"autopilot.redundancy_zone_tag"},
			badKeys: []string{"autopilot.redundancy_zone_tag"},
		},
		"autopilot.upgrade_version_tag": {
			config: Config{
				Autopilot: Autopilot{
					UpgradeVersionTag: &stringVal,
				},
			},
			keys:    []string{"autopilot.upgrade_version_tag"},
			badKeys: []string{"autopilot.upgrade_version_tag"},
		},
		"autopilot.disable_upgrade_migration": {
			config: Config{
				Autopilot: Autopilot{
					DisableUpgradeMigration: &boolVal,
				},
			},
			keys:    []string{"autopilot.disable_upgrade_migration"},
			badKeys: []string{"autopilot.disable_upgrade_migration"},
		},
		"dns_config.prefer_namespace": {
			config: Config{
				DNS: DNS{
					PreferNamespace: &boolVal,
				},
			},
			keys:    []string{"dns_config.prefer_namespace"},
			badKeys: []string{"dns_config.prefer_namespace"},
			check: func(t *testing.T, c *Config) {
				require.Nil(t, c.DNS.PreferNamespace)
			},
		},
		"acl.msp_disable_bootstrap": {
			config: Config{
				ACL: ACL{
					MSPDisableBootstrap: &boolVal,
				},
			},
			keys:    []string{"acl.msp_disable_bootstrap"},
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
			keys:    []string{"acl.tokens.managed_service_provider"},
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
			keys:    []string{"license_path"},
			badKeys: []string{"license_path"},
			check: func(t *testing.T, c *Config) {
				require.Empty(t, c.LicensePath)
			},
		},
		"multi": {
			config: Config{
				NonVotingServer: &boolVal,
				SegmentName:     &stringVal,
			},
			keys:    []string{"non_voting_server", "segment", "acl.tokens.agent_master"},
			badKeys: []string{"non_voting_server", "segment"},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			b := &Builder{}

			err := b.validateEnterpriseConfigKeys(&tcase.config, tcase.keys)
			if len(tcase.badKeys) > 0 {
				require.Error(t, err)

				multiErr, ok := err.(*multierror.Error)
				require.True(t, ok)

				var badKeys []string
				for _, e := range multiErr.Errors {
					if keyErr, ok := e.(enterpriseConfigKeyError); ok {
						badKeys = append(badKeys, keyErr.key)
						require.Contains(t, b.Warnings, keyErr.Error())
					}
				}

				require.ElementsMatch(t, tcase.badKeys, badKeys)

				if tcase.check != nil {
					tcase.check(t, &tcase.config)
				}

			} else {
				require.NoError(t, err)
			}
		})
	}
}
