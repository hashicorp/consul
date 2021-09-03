package config

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_DeprecatedConfig(t *testing.T) {
	opts := LoadOpts{
		HCL: []string{`
data_dir = "/foo"

acl_datacenter = "dcone"

acl_agent_master_token = "token1"
acl_agent_token = "token2"
acl_token = "token3"

`},
	}
	patchLoadOptsShims(&opts)
	result, err := Load(opts)
	require.NoError(t, err)

	expectWarns := []string{
		deprecationWarning("acl_agent_master_token", "acl.tokens.agent_master"),
		deprecationWarning("acl_agent_token", "acl.tokens.agent"),
		deprecationWarning("acl_datacenter", "primary_datacenter"),
		deprecationWarning("acl_token", "acl.tokens.default"),
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
	require.Equal(t, "token1", rt.ACLTokens.ACLAgentMasterToken)
	require.Equal(t, "token2", rt.ACLTokens.ACLAgentToken)
	require.Equal(t, "token3", rt.ACLTokens.ACLDefaultToken)
}
