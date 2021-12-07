package config

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad_DeprecatedConfig(t *testing.T) {
	opts := LoadOpts{
		HCL: []string{`
data_dir = "/foo"

acl_datacenter = "dcone"

acl_agent_token = "token1"
acl_token = "token2"

acl_replication_token = "token3"

acl_default_policy = "deny"
acl_down_policy = "async-cache"

acl_ttl = "3h"
acl_enable_key_list_policy = true

`},
	}
	patchLoadOptsShims(&opts)
	result, err := Load(opts)
	require.NoError(t, err)

	expectWarns := []string{
		deprecationWarning("acl_agent_token", "acl.tokens.agent"),
		deprecationWarning("acl_datacenter", "primary_datacenter"),
		deprecationWarning("acl_default_policy", "acl.default_policy"),
		deprecationWarning("acl_down_policy", "acl.down_policy"),
		deprecationWarning("acl_enable_key_list_policy", "acl.enable_key_list_policy"),
		deprecationWarning("acl_replication_token", "acl.tokens.replication"),
		deprecationWarning("acl_token", "acl.tokens.default"),
		deprecationWarning("acl_ttl", "acl.token_ttl"),
	}
	sort.Strings(result.Warnings)
	require.Equal(t, expectWarns, result.Warnings)
	// Ideally this would compare against the entire result.RuntimeConfig, but
	// we have so many non-zero defaults in that response that the noise of those
	// defaults makes this test difficult to read. So as a workaround, compare
	// specific values.
	rt := result.RuntimeConfig
	require.Equal(t, true, rt.ACLsEnabled)
	require.Equal(t, "dcone", rt.PrimaryDatacenter)
	require.Equal(t, "token1", rt.ACLTokens.ACLAgentToken)
	require.Equal(t, "token2", rt.ACLTokens.ACLDefaultToken)
	require.Equal(t, "token3", rt.ACLTokens.ACLReplicationToken)
	require.Equal(t, "deny", rt.ACLResolverSettings.ACLDefaultPolicy)
	require.Equal(t, "async-cache", rt.ACLResolverSettings.ACLDownPolicy)
	require.Equal(t, 3*time.Hour, rt.ACLResolverSettings.ACLTokenTTL)
	require.Equal(t, true, rt.ACLEnableKeyListPolicy)
}

func TestLoad_DeprecatedConfig_ACLReplication(t *testing.T) {
	opts := LoadOpts{
		HCL: []string{`
data_dir = "/foo"

enable_acl_replication = true

`},
	}
	patchLoadOptsShims(&opts)
	result, err := Load(opts)
	require.NoError(t, err)

	expectWarns := []string{
		deprecationWarning("enable_acl_replication", "acl.enable_token_replication"),
	}
	sort.Strings(result.Warnings)
	require.Equal(t, expectWarns, result.Warnings)
	// Ideally this would compare against the entire result.RuntimeConfig, but
	// we have so many non-zero defaults in that response that the noise of those
	// defaults makes this test difficult to read. So as a workaround, compare
	// specific values.
	rt := result.RuntimeConfig
	require.Equal(t, true, rt.ACLTokenReplication)
}

func TestLoad_DeprecatedConfig_ACLMasterTokens(t *testing.T) {
	t.Run("top-level fields", func(t *testing.T) {
		require := require.New(t)

		opts := LoadOpts{
			HCL: []string{`
				data_dir = "/foo"

				acl_master_token = "token1"
				acl_agent_master_token = "token2"
			`},
		}
		patchLoadOptsShims(&opts)

		result, err := Load(opts)
		require.NoError(err)

		expectWarns := []string{
			deprecationWarning("acl_master_token", "acl.tokens.initial_management"),
			deprecationWarning("acl_agent_master_token", "acl.tokens.agent_recovery"),
		}
		require.ElementsMatch(expectWarns, result.Warnings)

		rt := result.RuntimeConfig
		require.Equal("token1", rt.ACLMasterToken)
		require.Equal("token2", rt.ACLTokens.ACLAgentRecoveryToken)
	})

	t.Run("embedded in tokens struct", func(t *testing.T) {
		require := require.New(t)

		opts := LoadOpts{
			HCL: []string{`
				data_dir = "/foo"

				acl {
					tokens {
						master = "token1"
						agent_master = "token2"
					}
				}
			`},
		}
		patchLoadOptsShims(&opts)

		result, err := Load(opts)
		require.NoError(err)

		expectWarns := []string{
			deprecationWarning("acl.tokens.master", "acl.tokens.initial_management"),
			deprecationWarning("acl.tokens.agent_master", "acl.tokens.agent_recovery"),
		}
		require.ElementsMatch(expectWarns, result.Warnings)

		rt := result.RuntimeConfig
		require.Equal("token1", rt.ACLMasterToken)
		require.Equal("token2", rt.ACLTokens.ACLAgentRecoveryToken)
	})

	t.Run("both", func(t *testing.T) {
		require := require.New(t)

		opts := LoadOpts{
			HCL: []string{`
				data_dir = "/foo"

				acl_master_token = "token1"
				acl_agent_master_token = "token2"

				acl {
					tokens {
						master = "token3"
						agent_master = "token4"
					}
				}
			`},
		}
		patchLoadOptsShims(&opts)

		result, err := Load(opts)
		require.NoError(err)

		rt := result.RuntimeConfig
		require.Equal("token3", rt.ACLMasterToken)
		require.Equal("token4", rt.ACLTokens.ACLAgentRecoveryToken)
	})
}
