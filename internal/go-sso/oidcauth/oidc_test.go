// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidcauth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRSAPrivateKey = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDVMMi3HiDYhYmD\nbRi1MmacojGKP5HZMp4whUwp0oI+M0hpu2zQGv+p/vxUrsQQ6Esgp+sYA8aPRDky\ndMNLR+f0gjkAT7KglCIB6M4JfoLHaKrwCUPngQYWqdeVhl5abmRms/+gkZhqkkXr\nc3ax9yOCoyWMhaJjZeFyaeKt+DFDBB/VE8xNB5pVfCPgDJx5lmFwRtvzue65HLE6\nJHq8mA+y7k8qlH4H/yj5c1sZhJVUVxA/ixlDcYVI2vuPyoUAQsgUr4ZwlRVbbXOI\nUTMvnfy7OnScW0FxVNc66+7tp/qeTLBdkdMLa68hym/WbUnNqFbq7woraBoi+b+M\nNp1Uz5+7AgMBAAECggEATttsKvvcd2qxqmkEziVV+lMuUu5fswD7rYPo38Frhrlu\nbBm1Tqbl8coNKP+6K2zZOTuThL8Ex8KbC5RQFr0CyhkPH5PbRXV1vNIRwEZI9py7\nOe2bbfr2NxTc1wSsSvPxdGHZSNoCEE2JymVbvsllG7HgNkHKBs1NHoaXH/WhtyEX\nFoi2zEAl1xP3nrO6iJ/1Zjz0AHj+Ut0IL2abbT4ktQ4gkoSRjh7QMnBkQ4X5pyaM\nnQ1xhCMw8ryaV7zzCk5TuHiY2on1mp3F9TTq/lnyy712tY9g55IhX1vFu2iQ8Cv7\nfvOwZNvaxFJQVrs+kP3GZISEb34OvrKPycAN6lBEtQKBgQDrAXbFVW8TmPc38MjU\n/qEBfvzjzyUz9dGwuPK2y4ht1RdqYjT6n09FHTMUEcz+QxCTleAoZbI4TibAIWqG\nWu7HhjwEF0tIyXiEoUEWVhmbsPwbBc0yYFTJ3EhsyzLHwJ+tC0CSK5WTGygXpj6M\n1ZcpPjiiHDVpx/UQGtwKqvAk/wKBgQDoPGlgyKUTjpa6yj3Y2TMZnmWI4nJOBh7o\nEDX5vOhj7tfqrINllD6t4NFJFcVA30UK7RhmE0PkFAxnx/9+/+E0fUbRxFCNDGv5\nfVa6XaTqAwBsniGObkDjbzeNvRMloD3UzxeFdVkRVObXxJj7tLQ3ZymkYABBU3g7\nbEPt/cdZRQKBgEbKtBqRt9oxdBdX40e2RI4M0OVXGx/h5v7TV9oUyc48KMeVOdxd\nbSWmvCJJknTtgurSdSn2KI+piybJait67P8RwraAxd7xQerCILc3zJMH54nEX6HT\nPvdn8jFDrNJbhj48a4Ecu/wKbDNjkugd12FHKww6bySkZYAqdyqHf7vFAoGBAIXb\n5GWL4VKPeqP51II8V27p1N58n6QHdSMPzPzA/TY0wjGa9DXFqAczMY6txL+qsbIl\njU2wxw4c3DWpmsQKGzXVC8/3FvLl+QqaSzYqqdbUmhcBYpglRrORNHU3SWUDowAZ\nyhX72LXbuR8fS4qx0rqodOExEJSW1xNxSQpRn+j9AoGAXk6U9md5J3iRHtAPRIJI\nuWNm0rkBJnxBmxWVBlqghP5kXS6RGj72BlJjjT2nrbJeXjvHvBf31LHG+RrLuJUr\nl+P1QU5tkz6x487/yss7ZDkWgLoBuJmuVaTr8yQ548NJ48fuQEGRmQTtk/hXRqRp\n4keWXLkzwcqw6VF5MjfHaWA=\n-----END PRIVATE KEY-----\n"
const badRSAPrivateKey = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgk\n-----END PRIVATE KEY-----\n"

func setupForOIDC(t *testing.T) (*Authenticator, *oidcauthtest.Server) {
	t.Helper()

	srv := oidcauthtest.Start(t)
	srv.SetClientCreds("abc", "def")

	config := &Config{
		Type:                TypeOIDC,
		OIDCDiscoveryURL:    srv.Addr(),
		OIDCDiscoveryCACert: srv.CACert(),
		OIDCClientID:        "abc",
		OIDCClientSecret:    "def",
		OIDCACRValues:       []string{"acr1", "acr2"},
		JWTSupportedAlgs:    []string{"ES256"},
		BoundAudiences:      []string{"abc"},
		AllowedRedirectURIs: []string{"https://example.com"},
		ClaimMappings: map[string]string{
			"COLOR":            "color",
			"/nested/Size":     "size",
			"Age":              "age",
			"Admin":            "is_admin",
			"/nested/division": "division",
			"/nested/remote":   "is_remote",
			"flavor":           "flavor", // userinfo
		},
		ListClaimMappings: map[string]string{
			"/nested/Groups": "groups",
		},
	}

	require.NoError(t, config.Validate())

	oa, err := New(config, hclog.NewNullLogger())
	require.NoError(t, err)
	t.Cleanup(oa.Stop)

	return oa, srv
}

func TestOIDC_AuthURL(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		t.Parallel()

		oa, _ := setupForOIDC(t)

		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			map[string]string{"foo": "bar"},
		)
		require.NoError(t, err)

		require.True(t, strings.HasPrefix(authURL, oa.config.OIDCDiscoveryURL+"/auth?"))

		expected := map[string]string{
			"client_id":     "abc",
			"redirect_uri":  "https://example.com",
			"response_type": "code",
			"scope":         "openid",
			// optional values
			"acr_values": "acr1 acr2",
		}

		au, err := url.Parse(authURL)
		require.NoError(t, err)

		for k, v := range expected {
			assert.Equal(t, v, au.Query().Get(k), "key %q is incorrect", k)
		}

		assert.Regexp(t, `^[a-z0-9]{40}$`, au.Query().Get("nonce"))
		assert.Regexp(t, `^[a-z0-9]{40}$`, au.Query().Get("state"))

	})

	t.Run("invalid RedirectURI", func(t *testing.T) {
		t.Parallel()

		oa, _ := setupForOIDC(t)

		_, err := oa.GetAuthCodeURL(
			context.Background(),
			"http://bitc0in-4-less.cx",
			map[string]string{"foo": "bar"},
		)
		requireErrorContains(t, err, "unauthorized redirect_uri: http://bitc0in-4-less.cx")
	})

	t.Run("missing RedirectURI", func(t *testing.T) {
		t.Parallel()

		oa, _ := setupForOIDC(t)

		_, err := oa.GetAuthCodeURL(
			context.Background(),
			"",
			map[string]string{"foo": "bar"},
		)
		requireErrorContains(t, err, "missing redirect_uri")
	})

	t.Run("custom scopes and PKCE enabled/disabled", func(t *testing.T) {
		oa, _ := setupForOIDC(t)
		oa.config.OIDCScopes = []string{"profile", "email"}

		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			map[string]string{"foo": "bar"},
		)
		require.NoError(t, err)

		parsedURL, err := url.Parse(authURL)
		require.NoError(t, err)

		// Extract query parameters
		params := parsedURL.Query()
		require.Contains(t, authURL, "scope=openid+profile+email")
		require.NotContains(t, params, "code_challenge")
		require.Empty(t, params.Get("code_challenge"))

		oa.config.OIDCClientUsePKCE = new(bool) // PKCE enabled
		*oa.config.OIDCClientUsePKCE = true
		authURL2, _ := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			map[string]string{"foo": "bar"},
		)
		parsedURL, err = url.Parse(authURL2)
		require.NoError(t, err)

		// Extract query parameters
		params = parsedURL.Query()
		require.NoError(t, err)
		require.Contains(t, authURL2, "scope=openid+profile+email")
		require.Contains(t, params, "code_challenge")
		require.NotEmpty(t, params.Get("code_challenge"))
		require.Equal(t, "S256", params.Get("code_challenge_method"))
	})

	t.Run("oidc client assertion (private key JWT)", func(t *testing.T) {
		oa, _ := setupForOIDC(t)
		oa.config.OIDCClientAssertion = &OIDCClientAssertion{
			PrivateKey:   &OIDCClientAssertionKey{PemKey: testRSAPrivateKey},
			Audience:     []string{oa.config.OIDCDiscoveryURL},
			KeyAlgorithm: "RS256",
		}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			map[string]string{"foo": "bar"},
		)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(authURL, oa.config.OIDCDiscoveryURL+"/auth?"))

		expected := map[string]string{
			"client_id":     "abc",
			"redirect_uri":  "https://example.com",
			"response_type": "code",
			"scope":         "openid",
			// optional values
			"acr_values": "acr1 acr2",
		}

		au, err := url.Parse(authURL)
		require.NoError(t, err)

		for k, v := range expected {
			assert.Equal(t, v, au.Query().Get(k), "key %q is incorrect", k)
		}

		assert.Regexp(t, `^[a-z0-9]{40}$`, au.Query().Get("nonce"))
		assert.Regexp(t, `^[a-z0-9]{40}$`, au.Query().Get("state"))
	})

	t.Run("oidc client assertion invalid pemkey", func(t *testing.T) {
		oa, _ := setupForOIDC(t)
		oa.config.OIDCClientAssertion = &OIDCClientAssertion{
			PrivateKey:   &OIDCClientAssertionKey{PemKey: badRSAPrivateKey},
			Audience:     []string{oa.config.OIDCDiscoveryURL},
			KeyAlgorithm: "RS256",
		}
		_, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			map[string]string{"foo": "bar"},
		)
		requireErrorContains(t, err, "failed to parse RSA private key")
	})

	t.Run("unsupported key algorithm", func(t *testing.T) {
		oa, _ := setupForOIDC(t)

		oa.config.OIDCClientAssertion = &OIDCClientAssertion{
			PrivateKey:   &OIDCClientAssertionKey{PemKey: testRSAPrivateKey},
			Audience:     []string{oa.config.OIDCDiscoveryURL},
			KeyAlgorithm: "ES256",
		}
		origPayload := map[string]string{"foo": "bar"}
		_, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		requireErrorContains(t, err, "unsupported key algorithm")

	})
}

func TestOIDC_JWT_Functions_Fail(t *testing.T) {
	oa, srv := setupForOIDC(t)

	cl := jwt.Claims{
		Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
		Issuer:    srv.Addr(),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
		Audience:  jwt.Audience{"https://go-sso.test"},
	}

	privateCl := struct {
		User   string   `json:"https://go-sso/user"`
		Groups []string `json:"https://go-sso/groups"`
	}{
		"jeff",
		[]string{"foo", "bar"},
	}

	jwtData, err := oidcauthtest.SignJWT("", cl, privateCl)
	require.NoError(t, err)

	_, err = oa.ClaimsFromJWT(context.Background(), jwtData)
	requireErrorContains(t, err, `ClaimsFromJWT is incompatible with type "oidc"`)
}

func TestOIDC_ClaimsFromAuthCode(t *testing.T) {
	requireProviderError := func(t *testing.T, err error) {
		var provErr *ProviderLoginFailedError
		if !errors.As(err, &provErr) {
			t.Fatalf("error was not a *ProviderLoginFailedError")
		}
	}
	requireTokenVerificationError := func(t *testing.T, err error) {
		var tokErr *TokenVerificationFailedError
		if !errors.As(err, &tokErr) {
			t.Fatalf("error was not a *TokenVerificationFailedError")
		}
	}

	t.Run("successful login", func(t *testing.T) {
		oa, srv := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")
		nonce := getQueryParam(t, authURL, "nonce")

		// set provider claims that will be returned by the mock server
		srv.SetCustomClaims(sampleClaims(nonce))

		// set mock provider's expected code
		srv.SetExpectedAuthCode("abc")

		claims, payload, err := oa.ClaimsFromAuthCode(
			context.Background(),
			state, "abc",
		)
		require.NoError(t, err)

		require.Equal(t, origPayload, payload)

		expectedClaims := &Claims{
			Values: map[string]string{
				"color":     "green",
				"size":      "medium",
				"age":       "85",
				"is_admin":  "true",
				"division":  "3",
				"is_remote": "true",
				"flavor":    "umami", // from userinfo
			},
			Lists: map[string][]string{
				"groups": {"a", "b"},
			},
		}

		require.Equal(t, expectedClaims, claims)
	})

	t.Run("multiple and nested claim mappings", func(t *testing.T) {
		oa, srv := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")
		nonce := getQueryParam(t, authURL, "nonce")

		// set provider claims that will be returned by the mock server
		srv.SetCustomClaims(sampleClaims(nonce))

		// set mock provider's expected code
		srv.SetExpectedAuthCode("abc")

		oa.config.ClaimMappings = map[string]string{
			"email":        "user_email",
			"/nested/Size": "user_size",
		}
		oa.config.ListClaimMappings = map[string]string{
			"/nested/Groups": "groups",
		}

		srv.SetExpectedAuthCode("abc")

		// Now use mockState in your test
		resultClaims, _, err := oa.ClaimsFromAuthCode(
			context.Background(),
			state, "abc",
		)
		require.NoError(t, err)
		require.Equal(t, "bob@example.com", resultClaims.Values["user_email"])
		require.Equal(t, "medium", resultClaims.Values["user_size"])
		require.ElementsMatch(t, []string{"a", "b"}, resultClaims.Lists["groups"])
	})

	t.Run("State not found", func(t *testing.T) {
		oa, srv := setupForOIDC(t)
		nonce := "test-nonce"
		srv.SetCustomClaims(sampleClaims(nonce))
		srv.SetExpectedAuthCode("abc")

		_, _, err := oa.ClaimsFromAuthCode(
			context.Background(),
			"state", "abc",
		)
		requireErrorContains(t, err, "Expired or missing OAuth state")
	})

	t.Run("multiple audiences", func(t *testing.T) {
		oa, _ := setupForOIDC(t)
		oa.config.BoundAudiences = []string{"abc", "def"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			map[string]string{"foo": "bar"},
		)
		require.NoError(t, err)
		require.Contains(t, authURL, "client_id=abc")
		// Optionally: test with a token that matches one of the audiences
	})

	t.Run("failed login unusable claims", func(t *testing.T) {
		oa, srv := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")
		nonce := getQueryParam(t, authURL, "nonce")

		// set provider claims that will be returned by the mock server
		customClaims := sampleClaims(nonce)
		customClaims["COLOR"] = []interface{}{"yellow"}
		srv.SetCustomClaims(customClaims)

		// set mock provider's expected code
		srv.SetExpectedAuthCode("abc")

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			state, "abc",
		)
		requireErrorContains(t, err, "error converting claim 'COLOR' to string from unknown type []interface {}")
		requireTokenVerificationError(t, err)
	})

	t.Run("successful login - no userinfo", func(t *testing.T) {
		oa, srv := setupForOIDC(t)

		srv.DisableUserInfo()

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")
		nonce := getQueryParam(t, authURL, "nonce")

		// set provider claims that will be returned by the mock server
		srv.SetCustomClaims(sampleClaims(nonce))

		// set mock provider's expected code
		srv.SetExpectedAuthCode("abc")

		claims, payload, err := oa.ClaimsFromAuthCode(
			context.Background(),
			state, "abc",
		)
		require.NoError(t, err)

		require.Equal(t, origPayload, payload)

		expectedClaims := &Claims{
			Values: map[string]string{
				"color":     "green",
				"size":      "medium",
				"age":       "85",
				"is_admin":  "true",
				"division":  "3",
				"is_remote": "true",
				// "flavor":    "umami", // from userinfo
			},
			Lists: map[string][]string{
				"groups": {"a", "b"},
			},
		}

		require.Equal(t, expectedClaims, claims)
	})

	t.Run("failed login - bad nonce", func(t *testing.T) {
		t.Parallel()

		oa, srv := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")

		srv.SetCustomClaims(sampleClaims("bad nonce"))

		// set mock provider's expected code
		srv.SetExpectedAuthCode("abc")

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			state, "abc",
		)
		requireErrorContains(t, err, "invalid id_token nonce: invalid nonce")
		requireProviderError(t, err)
	})

	t.Run("missing state", func(t *testing.T) {
		oa, _ := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		_, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			"", "abc",
		)
		requireErrorContains(t, err, "Expired or missing OAuth state")
		requireProviderError(t, err)
	})

	t.Run("unknown state", func(t *testing.T) {
		oa, _ := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		_, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			"not_a_state", "abc",
		)
		requireErrorContains(t, err, "Expired or missing OAuth state")
		requireProviderError(t, err)
	})

	t.Run("valid state, missing code", func(t *testing.T) {
		oa, _ := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			state, "",
		)
		requireErrorContains(t, err, "OAuth code parameter not provided")
		requireProviderError(t, err)
	})

	t.Run("failed code exchange", func(t *testing.T) {
		oa, srv := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")

		// set mock provider's expected code
		srv.SetExpectedAuthCode("abc")

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			state, "wrong_code",
		)
		requireErrorContains(t, err, "Error exchanging oidc code")
		requireProviderError(t, err)
	})

	t.Run("no id_token returned", func(t *testing.T) {
		oa, srv := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")
		nonce := getQueryParam(t, authURL, "nonce")

		// set provider claims that will be returned by the mock server
		srv.SetCustomClaims(sampleClaims(nonce))

		// set mock provider's expected code
		srv.SetExpectedAuthCode("abc")

		srv.OmitIDTokens()

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			state, "abc",
		)
		requireErrorContains(t, err, "id_token is missing from auth code exchange")
		requireProviderError(t, err)
	})

	t.Run("no response from provider", func(t *testing.T) {
		oa, srv := setupForOIDC(t)

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")

		// close the server prematurely
		srv.Stop()
		srv.SetExpectedAuthCode("abc")

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			state, "abc",
		)
		requireErrorContains(t, err, "connection refused")
		requireProviderError(t, err)
	})

	t.Run("invalid bound audience", func(t *testing.T) {
		oa, srv := setupForOIDC(t)

		srv.SetClientCreds("not_gonna_match", "def")

		origPayload := map[string]string{"foo": "bar"}
		authURL, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			origPayload,
		)
		require.NoError(t, err)

		state := getQueryParam(t, authURL, "state")
		nonce := getQueryParam(t, authURL, "nonce")

		// set provider claims that will be returned by the mock server
		srv.SetCustomClaims(sampleClaims(nonce))

		// set mock provider's expected code
		srv.SetExpectedAuthCode("abc")

		_, _, err = oa.ClaimsFromAuthCode(
			context.Background(),
			state, "abc",
		)
		requireErrorContains(t, err, `invalid id_token audiences`)
		requireProviderError(t, err)
	})
}

func sampleClaims(nonce string) map[string]interface{} {
	return map[string]interface{}{
		"nonce": nonce,
		"email": "bob@example.com",
		"COLOR": "green",
		"sk":    "42",
		"Age":   85,
		"Admin": true,
		"nested": map[string]interface{}{
			"Size":        "medium",
			"division":    3,
			"remote":      true,
			"Groups":      []string{"a", "b"},
			"secret_code": "bar",
		},
		"password": "foo",
	}
}

func getQueryParam(t *testing.T, inputURL, param string) string {
	t.Helper()

	// Replace this function with one that properly parses full URLs
	u, err := url.Parse(inputURL)
	if err != nil {
		t.Fatalf("Failed to parse URL %q: %v", inputURL, err)
	}

	v := u.Query().Get(param)
	if v == "" {
		t.Fatalf("Query param %q not found in URL", param)
	}
	return v
}
