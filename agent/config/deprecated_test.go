package config

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
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

ca_file = "some-ca-file"
ca_path = "some-ca-path"
cert_file = "some-cert-file"
key_file = "some-key-file"
tls_cipher_suites = "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA"
tls_min_version = "tls11"
verify_incoming = true
verify_incoming_https = false
verify_incoming_rpc = false
verify_outgoing = true
verify_server_hostname = true
tls_prefer_server_cipher_suites = true
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
		deprecationWarning("ca_file", "tls.defaults.ca_file"),
		deprecationWarning("ca_path", "tls.defaults.ca_path"),
		deprecationWarning("cert_file", "tls.defaults.cert_file"),
		deprecationWarning("key_file", "tls.defaults.key_file"),
		deprecationWarning("tls_cipher_suites", "tls.defaults.tls_cipher_suites"),
		fmt.Sprintf("'tls_min_version' value 'tls11' is deprecated, please specify 'TLSv1_1' instead"),
		deprecationWarning("tls_min_version", "tls.defaults.tls_min_version"),
		deprecationWarning("verify_incoming", "tls.defaults.verify_incoming"),
		deprecationWarning("verify_incoming_https", "tls.https.verify_incoming"),
		deprecationWarning("verify_incoming_rpc", "tls.internal_rpc.verify_incoming"),
		deprecationWarning("verify_outgoing", "tls.defaults.verify_outgoing"),
		deprecationWarning("verify_server_hostname", "tls.internal_rpc.verify_server_hostname"),
		"The 'tls_prefer_server_cipher_suites' field is deprecated and will be ignored.",
	}
	require.ElementsMatch(t, expectWarns, result.Warnings)
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

	for _, l := range []tlsutil.ProtocolConfig{rt.TLS.InternalRPC, rt.TLS.GRPC, rt.TLS.HTTPS} {
		require.Equal(t, "some-ca-file", l.CAFile)
		require.Equal(t, "some-ca-path", l.CAPath)
		require.Equal(t, "some-cert-file", l.CertFile)
		require.Equal(t, "some-key-file", l.KeyFile)
		require.Equal(t, types.TLSVersion("TLSv1_1"), l.TLSMinVersion)
		require.Equal(t, []types.TLSCipherSuite{types.TLSCipherSuite("TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA")}, l.CipherSuites)
	}

	require.False(t, rt.TLS.InternalRPC.VerifyIncoming)
	require.False(t, rt.TLS.HTTPS.VerifyIncoming)
	require.False(t, rt.TLS.GRPC.VerifyIncoming)
	require.True(t, rt.TLS.InternalRPC.VerifyOutgoing)
	require.True(t, rt.TLS.HTTPS.VerifyOutgoing)
	require.True(t, rt.TLS.InternalRPC.VerifyServerHostname)
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

		opts := LoadOpts{
			HCL: []string{`
				data_dir = "/foo"

				acl_master_token = "token1"
				acl_agent_master_token = "token2"
			`},
		}
		patchLoadOptsShims(&opts)

		result, err := Load(opts)
		require.NoError(t, err)

		expectWarns := []string{
			deprecationWarning("acl_master_token", "acl.tokens.initial_management"),
			deprecationWarning("acl_agent_master_token", "acl.tokens.agent_recovery"),
		}
		require.ElementsMatch(t, expectWarns, result.Warnings)

		rt := result.RuntimeConfig
		require.Equal(t, "token1", rt.ACLInitialManagementToken)
		require.Equal(t, "token2", rt.ACLTokens.ACLAgentRecoveryToken)
	})

	t.Run("embedded in tokens struct", func(t *testing.T) {

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
		require.NoError(t, err)

		expectWarns := []string{
			deprecationWarning("acl.tokens.master", "acl.tokens.initial_management"),
			deprecationWarning("acl.tokens.agent_master", "acl.tokens.agent_recovery"),
		}
		require.ElementsMatch(t, expectWarns, result.Warnings)

		rt := result.RuntimeConfig
		require.Equal(t, "token1", rt.ACLInitialManagementToken)
		require.Equal(t, "token2", rt.ACLTokens.ACLAgentRecoveryToken)
	})

	t.Run("both", func(t *testing.T) {

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
		require.NoError(t, err)

		rt := result.RuntimeConfig
		require.Equal(t, "token3", rt.ACLInitialManagementToken)
		require.Equal(t, "token4", rt.ACLTokens.ACLAgentRecoveryToken)
	})
}
