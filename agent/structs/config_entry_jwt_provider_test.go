package structs

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/stretchr/testify/require"
)

func newTestAuthz(t *testing.T, src string) acl.Authorizer {
	policy, err := acl.NewPolicyFromSource(src, nil, nil)
	require.NoError(t, err)

	authorizer, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)
	return authorizer
}

func TestJWTProviderConfigEntry_ValidateAndNormalize(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"valid jwt-provider": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "test-jwt-provider",
				JSONWebKeySet: &JSONWebKeySet{
					Local: &LocalJWKS{
						Filename: "jwks.txt",
					},
				},
				ClockSkewSeconds: 0,
			},
			expected: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "test-jwt-provider",
				JSONWebKeySet: &JSONWebKeySet{
					Local: &LocalJWKS{
						Filename: "jwks.txt",
					},
				},
				// TODO RONALD - update this
				// ClockSkewSeconds: defaultClockSkewSeconds,
				ClockSkewSeconds: 0,
			},
		},
		"invalid jwt-provider - no name": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "",
			},
			validateErr: "name is required",
		},
		"invalid jwt-provider - no jwks": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
			},
			validateErr: "JSONWebKeySet is required",
		},
		"invalid jwt-provider - no jwks local or remote set": {
			entry: &JWTProviderConfigEntry{
				Kind:          JWTProvider,
				Name:          "okta",
				JSONWebKeySet: &JSONWebKeySet{},
			},
			validateErr: "must specify exactly one of Local or Remote JSON Web key set",
		},
		"invalid jwt-provider - both jwks local and remote set": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Local: &LocalJWKS{
						Filename: "jwks.txt",
					},
					Remote: &RemoteJWKS{},
				},
			},
			validateErr: "must specify exactly one of Local or Remote JSON Web key set",
		},
	}

	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestJWTProviderConfigEntry_ACLs(t *testing.T) {
	type testACL = configEntryTestACL

	cases := []configEntryACLTestCase{
		{
			name: "jwt-provider",
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "test-provider",
				JSONWebKeySet: &JSONWebKeySet{
					Local: &LocalJWKS{
						Filename: "jwks.txt",
					},
				},
			},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newTestAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "jwt-provider: mesh read",
					authorizer: newTestAuthz(t, `mesh = "read"`),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "jwt-provider: mesh write",
					authorizer: newTestAuthz(t, `mesh = "write"`),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
	}
	testConfigEntries_ListRelatedServices_AndACLs(t, cases)
}
