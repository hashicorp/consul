// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package oidcauth

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOIDC_ClaimsFromAuthCodeWithIDToken verifies that the WithIDToken variant
// returns the same claims/payload as ClaimsFromAuthCode and additionally
// surfaces the raw, UNREDACTED id_token so it can be used as an id_token_hint
// for front-channel logout.
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
	// placeholder ("[REDACTED: id_token]"), otherwise front-channel logout
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
