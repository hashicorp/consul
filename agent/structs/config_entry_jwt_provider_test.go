package structs

import (
	"testing"
)

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
				ClockSkewSeconds: defaultClockSkewSeconds,
			},
		},
		"invalid jwt-provider - no name": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "",
			},
			validateErr: "Name is required",
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
