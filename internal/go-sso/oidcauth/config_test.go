// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package oidcauth

import (
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	type testcase struct {
		config         Config
		expectAuthType int
		expectErr      string
	}

	srv := oidcauthtest.Start(t)

	oidcCases := map[string]testcase{
		"all required": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
			},
			expectAuthType: authOIDCFlow,
		},
		"missing required OIDCDiscoveryURL": {
			config: Config{
				Type: TypeOIDC,
				// OIDCDiscoveryURL:    srv.Addr(),
				// OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
			},
			expectErr: "must be set for type",
		},
		"missing required OIDCClientID": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				// OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
			},
			expectErr: "must be set for type",
		},
		"missing required OIDCClientSecret": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				// OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
			},
			expectErr: "must be set for type",
		},
		"missing required AllowedRedirectURIs": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{},
			},
			expectErr: "must be set for type",
		},
		"incompatible with JWKSURL": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				JWKSURL:             srv.Addr() + "/certs",
			},
			expectErr: "must not be set for type",
		},
		"incompatible with JWKSCACert": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				JWKSCACert:          srv.CACert(),
			},
			expectErr: "must not be set for type",
		},
		"incompatible with JWTValidationPubKeys": {
			config: Config{
				Type:                 TypeOIDC,
				OIDCDiscoveryURL:     srv.Addr(),
				OIDCDiscoveryCACert:  srv.CACert(),
				OIDCClientID:         "abc",
				OIDCClientSecret:     "def",
				AllowedRedirectURIs:  []string{"http://foo.test"},
				JWTValidationPubKeys: []string{testJWTPubKey},
			},
			expectErr: "must not be set for type",
		},
		"incompatible with BoundIssuer": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				BoundIssuer:         "foo",
			},
			expectErr: "must not be set for type",
		},
		"incompatible with ExpirationLeeway": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				ExpirationLeeway:    1 * time.Second,
			},
			expectErr: "must not be set for type",
		},
		"incompatible with NotBeforeLeeway": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				NotBeforeLeeway:     1 * time.Second,
			},
			expectErr: "must not be set for type",
		},
		"incompatible with ClockSkewLeeway": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				ClockSkewLeeway:     1 * time.Second,
			},
			expectErr: "must not be set for type",
		},
		"bad discovery cert": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: oidcBadCACerts,
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
			},
			expectErr: "certificate signed by unknown authority",
		},
		"garbage discovery cert": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: garbageCACert,
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
			},
			expectErr: "could not parse CA PEM value successfully",
		},
		"good discovery cert": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
			},
			expectAuthType: authOIDCFlow,
		},
		"valid redirect uris": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{
					"http://foo.test",
					"https://example.com",
					"https://evilcorp.com:8443",
				},
			},
			expectAuthType: authOIDCFlow,
		},
		"invalid redirect uris": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{
					"%%%%",
					"http://foo.test",
					"https://example.com",
					"https://evilcorp.com:8443",
				},
			},
			expectErr: "Invalid AllowedRedirectURIs provided: [%%%%]",
		},
		"valid algorithm": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				JWTSupportedAlgs: []string{
					oidc.RS256, oidc.RS384, oidc.RS512,
					oidc.ES256, oidc.ES384, oidc.ES512,
					oidc.PS256, oidc.PS384, oidc.PS512,
				},
			},
			expectAuthType: authOIDCFlow,
		},
		"invalid algorithm": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				JWTSupportedAlgs: []string{
					oidc.RS256, oidc.RS384, oidc.RS512,
					oidc.ES256, oidc.ES384, oidc.ES512,
					oidc.PS256, oidc.PS384, oidc.PS512,
					"foo",
				},
			},
			expectErr: "Invalid supported algorithm",
		},
		"valid claim mappings": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				ClaimMappings: map[string]string{
					"foo":          "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
				ListClaimMappings: map[string]string{
					"foo":          "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
			},
			expectAuthType: authOIDCFlow,
		},
		"invalid repeated value claim mappings": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				ClaimMappings: map[string]string{
					"foo":          "bar",
					"bling":        "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
				ListClaimMappings: map[string]string{
					"foo":          "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
			},
			expectErr: "ClaimMappings contains multiple mappings for key",
		},
		"invalid repeated list claim mappings": {
			config: Config{
				Type:                TypeOIDC,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				OIDCClientID:        "abc",
				OIDCClientSecret:    "def",
				AllowedRedirectURIs: []string{"http://foo.test"},
				ClaimMappings: map[string]string{
					"foo":          "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
				ListClaimMappings: map[string]string{
					"foo":          "bar",
					"bling":        "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
			},
			expectErr: "ListClaimMappings contains multiple mappings for key",
		},
	}

	jwtCases := map[string]testcase{
		"all required for oidc discovery": {
			config: Config{
				Type:                TypeJWT,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
			},
			expectAuthType: authOIDCDiscovery,
		},
		"all required for jwks": {
			config: Config{
				Type:       TypeJWT,
				JWKSURL:    srv.Addr() + "/certs",
				JWKSCACert: srv.CACert(), // needed to avoid self signed cert issue
			},
			expectAuthType: authJWKS,
		},
		"all required for public keys": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
			},
			expectAuthType: authStaticKeys,
		},
		"incompatible with OIDCClientID": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				OIDCClientID:         "abc",
			},
			expectErr: "must not be set for type",
		},
		"incompatible with OIDCClientSecret": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				OIDCClientSecret:     "abc",
			},
			expectErr: "must not be set for type",
		},
		"incompatible with OIDCScopes": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				OIDCScopes:           []string{"blah"},
			},
			expectErr: "must not be set for type",
		},
		"incompatible with OIDCACRValues": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				OIDCACRValues:        []string{"acr1"},
			},
			expectErr: "must not be set for type",
		},
		"incompatible with AllowedRedirectURIs": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				AllowedRedirectURIs:  []string{"http://foo.test"},
			},
			expectErr: "must not be set for type",
		},
		"incompatible with VerboseOIDCLogging": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				VerboseOIDCLogging:   true,
			},
			expectErr: "must not be set for type",
		},
		"too many methods (discovery + jwks)": {
			config: Config{
				Type:                TypeJWT,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				JWKSURL:             srv.Addr() + "/certs",
				JWKSCACert:          srv.CACert(),
				// JWTValidationPubKeys: []string{testJWTPubKey},
			},
			expectErr: "exactly one of",
		},
		"too many methods (discovery + pubkeys)": {
			config: Config{
				Type:                TypeJWT,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				// JWKSURL:          srv.Addr() + "/certs",
				// JWKSCACert:       srv.CACert(),
				JWTValidationPubKeys: []string{testJWTPubKey},
			},
			expectErr: "exactly one of",
		},
		"too many methods (jwks + pubkeys)": {
			config: Config{
				Type: TypeJWT,
				// OIDCDiscoveryURL:     srv.Addr(),
				// OIDCDiscoveryCACert:  srv.CACert(),
				JWKSURL:              srv.Addr() + "/certs",
				JWKSCACert:           srv.CACert(),
				JWTValidationPubKeys: []string{testJWTPubKey},
			},
			expectErr: "exactly one of",
		},
		"too many methods (discovery + jwks + pubkeys)": {
			config: Config{
				Type:                 TypeJWT,
				OIDCDiscoveryURL:     srv.Addr(),
				OIDCDiscoveryCACert:  srv.CACert(),
				JWKSURL:              srv.Addr() + "/certs",
				JWKSCACert:           srv.CACert(),
				JWTValidationPubKeys: []string{testJWTPubKey},
			},
			expectErr: "exactly one of",
		},
		"incompatible with JWKSCACert": {
			config: Config{
				Type:                TypeJWT,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
				JWKSCACert:          srv.CACert(),
			},
			expectErr: "should not be set unless",
		},
		"invalid pubkey": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKeyBad},
			},
			expectErr: "error parsing public key",
		},
		"incompatible with OIDCDiscoveryCACert": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				OIDCDiscoveryCACert:  srv.CACert(),
			},
			expectErr: "should not be set unless",
		},
		"bad discovery cert": {
			config: Config{
				Type:                TypeJWT,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: oidcBadCACerts,
			},
			expectErr: "certificate signed by unknown authority",
		},
		"good discovery cert": {
			config: Config{
				Type:                TypeJWT,
				OIDCDiscoveryURL:    srv.Addr(),
				OIDCDiscoveryCACert: srv.CACert(),
			},
			expectAuthType: authOIDCDiscovery,
		},
		"jwks invalid 404": {
			config: Config{
				Type:       TypeJWT,
				JWKSURL:    srv.Addr() + "/certs_missing",
				JWKSCACert: srv.CACert(),
			},
			expectErr: "get keys failed",
		},
		"jwks mismatched certs": {
			config: Config{
				Type:       TypeJWT,
				JWKSURL:    srv.Addr() + "/certs_invalid",
				JWKSCACert: srv.CACert(),
			},
			expectErr: "failed to decode keys",
		},
		"jwks bad certs": {
			config: Config{
				Type:       TypeJWT,
				JWKSURL:    srv.Addr() + "/certs_invalid",
				JWKSCACert: garbageCACert,
			},
			expectErr: "could not parse CA PEM value successfully",
		},
		"valid algorithm": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				JWTSupportedAlgs: []string{
					oidc.RS256, oidc.RS384, oidc.RS512,
					oidc.ES256, oidc.ES384, oidc.ES512,
					oidc.PS256, oidc.PS384, oidc.PS512,
				},
			},
			expectAuthType: authStaticKeys,
		},
		"invalid algorithm": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				JWTSupportedAlgs: []string{
					oidc.RS256, oidc.RS384, oidc.RS512,
					oidc.ES256, oidc.ES384, oidc.ES512,
					oidc.PS256, oidc.PS384, oidc.PS512,
					"foo",
				},
			},
			expectErr: "Invalid supported algorithm",
		},
		"valid claim mappings": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				ClaimMappings: map[string]string{
					"foo":          "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
				ListClaimMappings: map[string]string{
					"foo":          "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
			},
			expectAuthType: authStaticKeys,
		},
		"invalid repeated value claim mappings": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				ClaimMappings: map[string]string{
					"foo":          "bar",
					"bling":        "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
				ListClaimMappings: map[string]string{
					"foo":          "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
			},
			expectErr: "ClaimMappings contains multiple mappings for key",
		},
		"invalid repeated list claim mappings": {
			config: Config{
				Type:                 TypeJWT,
				JWTValidationPubKeys: []string{testJWTPubKey},
				ClaimMappings: map[string]string{
					"foo":          "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
				ListClaimMappings: map[string]string{
					"foo":          "bar",
					"bling":        "bar",
					"peanutbutter": "jelly",
					"wd40":         "ducttape",
				},
			},
			expectErr: "ListClaimMappings contains multiple mappings for key",
		},
	}

	cases := map[string]testcase{
		"bad type": {
			config:    Config{Type: "invalid"},
			expectErr: "authenticator type should be",
		},
	}

	for k, v := range oidcCases {
		cases["type=oidc/"+k] = v

		v2 := v
		v2.config.Type = ""
		cases["type=inferred_oidc/"+k] = v2
	}
	for k, v := range jwtCases {
		cases["type=jwt/"+k] = v
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectErr != "" {
				require.Error(t, err)
				requireErrorContains(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectAuthType, tc.config.authType())
			}
		})
	}
}

func requireErrorContains(t *testing.T, err error, expectedErrorMessage string) {
	t.Helper()
	if err == nil {
		t.Fatal("An error is expected but got nil.")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("unexpected error: %v", err)
	}
}

const (
	testJWTPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEEVs/o5+uQbTjL3chynL4wXgUg2R9
q9UU8I5mEovUf86QZ7kOBIjJwqnzD1omageEHWwHdBO6B+dFabmdT9POxg==
-----END PUBLIC KEY-----`

	testJWTPubKeyBad = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIrollingyourricksEVs/o5+uQbTjL3chynL4wXgUg2R9
q9UU8I5mEovUf86QZ7kOBIjJwqnzD1omageEHWwHdBO6B+dFabmdT9POxg==
-----END PUBLIC KEY-----`

	garbageCACert = `this is not a key`

	oidcBadCACerts = `-----BEGIN CERTIFICATE-----
MIIDYDCCAkigAwIBAgIJAK8uAVsPxWKGMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTgwNzA5MTgwODI5WhcNMjgwNzA2MTgwODI5WjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEA1eaEmIHKQqDlSadCtg6YY332qIMoeSb2iZTRhBRYBXRhMIKF3HoLXlI8
/3veheMnBQM7zxIeLwtJ4VuZVZcpJlqHdsXQVj6A8+8MlAzNh3+Xnv0tjZ83QLwZ
D6FWvMEzihxATD9uTCu2qRgeKnMYQFq4EG72AGb5094zfsXTAiwCfiRPVumiNbs4
Mr75vf+2DEhqZuyP7GR2n3BKzrWo62yAmgLQQ07zfd1u1buv8R72HCYXYpFul5qx
slZHU3yR+tLiBKOYB+C/VuB7hJZfVx25InIL1HTpIwWvmdk3QzpSpAGIAxWMXSzS
oRmBYGnsgR6WTymfXuokD4ZhHOpFZQIDAQABo1MwUTAdBgNVHQ4EFgQURh/QFJBn
hMXcgB1bWbGiU9B2VBQwHwYDVR0jBBgwFoAURh/QFJBnhMXcgB1bWbGiU9B2VBQw
DwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAr8CZLA3MQjMDWweS
ax9S1fRb8ifxZ4RqDcLj3dw5KZqnjEo8ggczR66T7vVXet/2TFBKYJAM0np26Z4A
WjZfrDT7/bHXseWQAUhw/k2d39o+Um4aXkGpg1Paky9D+ddMdbx1hFkYxDq6kYGd
PlBYSEiYQvVxDx7s7H0Yj9FWKO8WIO6BRUEvLlG7k/Xpp1OI6dV3nqwJ9CbcbqKt
ff4hAtoAmN0/x6yFclFFWX8s7bRGqmnoj39/r98kzeGFb/lPKgQjSVcBJuE7UO4k
8HP6vsnr/ruSlzUMv6XvHtT68kGC1qO3MfqiPhdSa4nxf9g/1xyBmAw/Uf90BJrm
sj9DpQ==
-----END CERTIFICATE-----`
)
