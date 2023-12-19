package oidcauth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"
)

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
		requireErrorContains(t, err, "Invalid ID token nonce")
		requireTokenVerificationError(t, err)
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
		requireErrorContains(t, err, "cannot fetch token")
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
		requireErrorContains(t, err, "No id_token found in response")
		requireTokenVerificationError(t, err)
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
		requireErrorContains(t, err, `error validating signature: oidc: expected audience "abc" got ["not_gonna_match"]`)
		requireTokenVerificationError(t, err)
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

	m, err := url.ParseQuery(inputURL)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := m[param]
	if !ok {
		t.Fatalf("query param %q not found", param)
	}
	return v[0]
}
