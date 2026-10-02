// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package oidcauth

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

// TestOIDC_ClaimsFromAuthCodeWithIDToken verifies that the WithIDToken variant
// returns the same claims/payload as ClaimsFromAuthCode and additionally
// surfaces the raw, UNREDACTED id_token so it can be used as an id_token_hint
// for RP-Initiated logout.
func TestOIDC_ClaimsFromAuthCodeWithIDToken(t *testing.T) {
	oa, srv := setupForOIDC(t)

	origPayload := map[string]string{"foo": "bar"}
	authURL, err := oa.GetAuthCodeURL(context.Background(), "https://example.com", origPayload)
	require.NoError(t, err)

	state := getQueryParam(t, authURL, "state")
	nonce := getQueryParam(t, authURL, "nonce")

	srv.SetCustomClaims(sampleClaims(nonce))
	srv.SetExpectedAuthCode("abc")

	claims, payload, rawIDToken, err := oa.ClaimsFromAuthCodeWithIDToken(context.Background(), state, "abc")
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.Equal(t, origPayload, payload)

	// The raw id_token must be the real JWT, not the cap library's redacted
	// placeholder ("[REDACTED: id_token]"), otherwise RP-Initiated logout
	// cannot pass a valid id_token_hint.
	require.NotEmpty(t, rawIDToken)
	require.NotContains(t, rawIDToken, "REDACTED")
	require.Len(t, strings.Split(rawIDToken, "."), 3, "id_token should be a three-part JWT")
}

// TestOIDC_ClaimsFromAuthCodeWithIDToken_TypeMismatch ensures the helper reports
// a clear error (not tied to a single method name) when used with a JWT config.
func TestOIDC_ClaimsFromAuthCodeWithIDToken_TypeMismatch(t *testing.T) {
	oa, _ := setupForJWT(t, authJWKS, nil)

	_, _, _, err := oa.ClaimsFromAuthCodeWithIDToken(context.Background(), "state", "code")
	require.Error(t, err)
	require.Contains(t, err.Error(), `auth code claims are incompatible with type "jwt"`)
}

// TestOIDC_GetEndSessionEndpoint verifies the end_session_endpoint is parsed
// from the provider's discovery document.
func TestOIDC_GetEndSessionEndpoint(t *testing.T) {
	oa, srv := setupForOIDC(t)

	endpoint, err := oa.GetEndSessionEndpoint()
	require.NoError(t, err)
	require.Equal(t, srv.Addr()+"/logout", endpoint)
}

// TestOIDC_GetEndSessionEndpoint_TypeMismatch ensures GetEndSessionEndpoint
// returns an error when called on a JWT-type authenticator.
func TestOIDC_GetEndSessionEndpoint_TypeMismatch(t *testing.T) {
	oa, _ := setupForJWT(t, authJWKS, nil)

	_, err := oa.GetEndSessionEndpoint()
	require.Error(t, err)
	require.Contains(t, err.Error(), "incompatible with type")
}

// TestOIDC_GetEndSessionEndpoint_NotAdvertised verifies the graceful backward-
// compatible no-op path: when the provider's discovery document does not
// advertise an end_session_endpoint, GetEndSessionEndpoint returns an empty
// string (and no error), so RP-Initiated logout is simply skipped and the
// login/logout flow behaves exactly as it did before this feature.
func TestOIDC_GetEndSessionEndpoint_NotAdvertised(t *testing.T) {
	srv := oidcauthtest.Start(t)
	srv.SetClientCreds("abc", "def")
	// Must be set before New() below: the discovery document is fetched and
	// cached when the authenticator (and its provider) is created.
	srv.DisableEndSession()

	config := &Config{
		Type:                TypeOIDC,
		OIDCDiscoveryURL:    srv.Addr(),
		OIDCDiscoveryCACert: srv.CACert(),
		OIDCClientID:        "abc",
		OIDCClientSecret:    "def",
		JWTSupportedAlgs:    []string{"ES256"},
		BoundAudiences:      []string{"abc"},
		AllowedRedirectURIs: []string{"https://example.com"},
	}
	require.NoError(t, config.Validate())

	oa, err := New(config, hclog.NewNullLogger())
	require.NoError(t, err)
	t.Cleanup(oa.Stop)

	endpoint, err := oa.GetEndSessionEndpoint()
	require.NoError(t, err)
	require.Empty(t, endpoint, "end_session_endpoint must be empty when the provider does not advertise one")
}
