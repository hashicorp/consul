// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"
	"time"

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

var tenSeconds time.Duration = 10 * time.Second
var hundredSeconds time.Duration = 100 * time.Second
var connectTimeout = time.Duration(5) * time.Second

func TestJWTProviderConfigEntry_ValidateAndNormalize(t *testing.T) {
	defaultMeta := DefaultEnterpriseMetaInDefaultPartition()

	cases := map[string]configEntryTestcase{
		"valid jwt-provider - local jwks": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "test-jwt-provider",
				JSONWebKeySet: &JSONWebKeySet{
					Local: &LocalJWKS{
						Filename: "jwks.txt",
					},
				},
			},
			expected: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "test-jwt-provider",
				JSONWebKeySet: &JSONWebKeySet{
					Local: &LocalJWKS{
						Filename: "jwks.txt",
					},
				},
				ClockSkewSeconds: DefaultClockSkewSeconds,
				EnterpriseMeta:   *defaultMeta,
			},
		},
		"valid jwt-provider - remote jwks defaults": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "test-jwt-provider",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
					},
				},
				Locations: []*JWTLocation{
					{
						Header: &JWTLocationHeader{
							Name: "Authorization",
						},
					},
				},
				Forwarding: &JWTForwardingConfig{
					HeaderName: "Some-Header",
				},
			},
			expected: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "test-jwt-provider",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
					},
				},
				Forwarding: &JWTForwardingConfig{
					HeaderName: "Some-Header",
				},
				Locations: []*JWTLocation{
					{
						Header: &JWTLocationHeader{
							Name: "Authorization",
						},
					},
				},
				ClockSkewSeconds: DefaultClockSkewSeconds,
				EnterpriseMeta:   *defaultMeta,
			},
		},
		"invalid jwt-provider - no name": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "",
			},
			validateErr: "Name is required",
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
		"invalid jwt-provider - local jwks with non-encoded base64 jwks": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Local: &LocalJWKS{
						JWKS: "not base64 encoded",
					},
				},
			},
			validateErr: "JWKS must be valid base64 encoded string",
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
		"invalid jwt-provider - local jwks string and filename both set": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Local: &LocalJWKS{
						Filename: "jwks.txt",
						JWKS:     "d2VhcmV0ZXN0aW5n",
					},
				},
			},
			validateErr: "must specify exactly one of String or filename for local keyset",
		},
		"invalid jwt-provider - remote jwks missing uri": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
					},
				},
			},
			validateErr: "Remote JWKS URI is required",
		},
		"invalid jwt-provider - remote jwks invalid uri": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "jibberishUrl",
					},
				},
			},
			validateErr: "Remote JWKS URI is invalid",
		},
		"invalid jwt-provider - JWT location with all fields": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
					},
				},
				Locations: []*JWTLocation{
					{
						Header: &JWTLocationHeader{
							Name: "Authorization",
						},
						QueryParam: &JWTLocationQueryParam{
							Name: "TOKEN-QUERY",
						},
						Cookie: &JWTLocationCookie{
							Name: "SomeCookie",
						},
					},
				},
			},
			validateErr: "must set exactly one of: JWT location header, query param or cookie",
		},
		"invalid jwt-provider - Remote JWKS retry policy maxinterval < baseInterval": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
						RetryPolicy: &JWKSRetryPolicy{
							RetryPolicyBackOff: &RetryPolicyBackOff{
								BaseInterval: hundredSeconds,
								MaxInterval:  tenSeconds,
							},
						},
					},
				},
			},
			validateErr: "retry policy backoff's MaxInterval should be greater or equal to BaseInterval",
		},
		"invalid jwt-provider - Remote JWKS cluster wrong discovery type": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
						JWKSCluster: &JWKSCluster{
							DiscoveryType: "FAKE",
						},
					},
				},
			},
			validateErr: "unsupported jwks cluster discovery type: \"FAKE\"",
		},
		"invalid jwt-provider - Remote JWKS cluster with both trustedCa and provider instance": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
						JWKSCluster: &JWKSCluster{
							TLSCertificates: &JWKSTLSCertificate{
								TrustedCa:                     &JWKSTLSCertTrustedCa{},
								CaCertificateProviderInstance: &JWKSTLSCertProviderInstance{},
							},
						},
					},
				},
			},
			validateErr: "must specify exactly one of: CaCertificateProviderInstance or TrustedCa for JKWS' TLSCertificates",
		},
		"invalid jwt-provider - Remote JWKS cluster with multiple trustedCa options": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
						JWKSCluster: &JWKSCluster{
							TLSCertificates: &JWKSTLSCertificate{
								TrustedCa: &JWKSTLSCertTrustedCa{
									Filename:     "myfile.cert",
									InlineString: "*****",
								},
							},
						},
					},
				},
			},
			validateErr: "must specify exactly one of: Filename, EnvironmentVariable, InlineString or InlineBytes for JWKS' TrustedCa",
		},
		"invalid jwt-provider - JWT location with 2 fields": {
			entry: &JWTProviderConfigEntry{
				Kind: JWTProvider,
				Name: "okta",
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
					},
				},
				Locations: []*JWTLocation{
					{
						Header: &JWTLocationHeader{
							Name: "Authorization",
						},
						QueryParam: &JWTLocationQueryParam{
							Name: "TOKEN-QUERY",
						},
					},
				},
			},
			validateErr: "must set exactly one of: JWT location header, query param or cookie",
		},
		"valid jwt-provider - with all possible fields": {
			entry: &JWTProviderConfigEntry{
				Kind:      JWTProvider,
				Name:      "test-jwt-provider",
				Issuer:    "iss",
				Audiences: []string{"api", "web"},
				CacheConfig: &JWTCacheConfig{
					Size: 30,
				},
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
						RetryPolicy: &JWKSRetryPolicy{
							RetryPolicyBackOff: &RetryPolicyBackOff{
								BaseInterval: tenSeconds,
								MaxInterval:  hundredSeconds,
							},
						},
						JWKSCluster: &JWKSCluster{
							DiscoveryType:  "STATIC",
							ConnectTimeout: connectTimeout,
							TLSCertificates: &JWKSTLSCertificate{
								TrustedCa: &JWKSTLSCertTrustedCa{
									Filename: "myfile.cert",
								},
							},
						},
					},
				},
				Forwarding: &JWTForwardingConfig{
					HeaderName: "Some-Header",
				},
				Locations: []*JWTLocation{
					{
						Cookie: &JWTLocationCookie{
							Name: "SomeCookie",
						},
					},
				},
				ClockSkewSeconds: 20,
			},
			expected: &JWTProviderConfigEntry{
				Kind:      JWTProvider,
				Name:      "test-jwt-provider",
				Issuer:    "iss",
				Audiences: []string{"api", "web"},
				CacheConfig: &JWTCacheConfig{
					Size: 30,
				},
				JSONWebKeySet: &JSONWebKeySet{
					Remote: &RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
						RetryPolicy: &JWKSRetryPolicy{
							RetryPolicyBackOff: &RetryPolicyBackOff{
								BaseInterval: tenSeconds,
								MaxInterval:  hundredSeconds,
							},
						},
						JWKSCluster: &JWKSCluster{
							DiscoveryType:  "STATIC",
							ConnectTimeout: connectTimeout,
							TLSCertificates: &JWKSTLSCertificate{
								TrustedCa: &JWKSTLSCertTrustedCa{
									Filename: "myfile.cert",
								},
							},
						},
					},
				},
				Forwarding: &JWTForwardingConfig{
					HeaderName: "Some-Header",
				},
				Locations: []*JWTLocation{
					{
						Cookie: &JWTLocationCookie{
							Name: "SomeCookie",
						},
					},
				},
				ClockSkewSeconds: 20,
				EnterpriseMeta:   *defaultMeta,
			},
		},
	}

	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestJWTProviderConfigEntry_ACLs(t *testing.T) {
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
			expectACLs: []configEntryTestACL{
				{
					name:       "no-authz",
					authorizer: newTestAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "jwt-provider: any service write",
					authorizer: newTestAuthz(t, `service "" { policy = "write" }`),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "jwt-provider: specific service write",
					authorizer: newTestAuthz(t, `service "web" { policy = "write" }`),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "jwt-provider: any service prefix write",
					authorizer: newTestAuthz(t, `service_prefix "" { policy = "write" }`),
					canRead:    true,
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
				{
					name:       "jwt-provider: operator read",
					authorizer: newTestAuthz(t, `operator = "read"`),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "jwt-provider: operator write",
					authorizer: newTestAuthz(t, `operator = "write"`),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
	}
	testConfigEntries_ListRelatedServices_AndACLs(t, cases)
}
