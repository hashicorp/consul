// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package jwtauth

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func encodeJWKS(t *testing.T) string {
	t.Helper()
	jwks, _ := makeJWKS(t)
	return base64.StdEncoding.EncodeToString([]byte(jwks))
}

func localProvider(name, jwks string) *api.JWTProviderConfigEntry {
	return &api.JWTProviderConfigEntry{
		Kind:          api.JWTProvider,
		Name:          name,
		Issuer:        "https://issuer.example.test/",
		Audiences:     []string{"consul-test"},
		JSONWebKeySet: &api.JSONWebKeySet{Local: &api.LocalJWKS{JWKS: jwks}},
	}
}

func setProvider(t *testing.T, c *api.Client, entry api.ConfigEntry) {
	t.Helper()
	ok, _, err := c.ConfigEntries().Set(entry, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

func getProvider(t *testing.T, c *api.Client, name string) *api.JWTProviderConfigEntry {
	t.Helper()
	entry, _, err := c.ConfigEntries().Get(api.JWTProvider, name, nil)
	require.NoError(t, err)
	return entry.(*api.JWTProviderConfigEntry)
}

// TestJWTProvider_ConfigEntry verifies how a jwt-provider entry is validated
// and stored: where keys come from, where tokens are looked for, and what is
// refused up front.
func TestJWTProvider_ConfigEntry(t *testing.T) {
	t.Parallel()

	c := newConfigClient(t)
	jwks := encodeJWKS(t)

	t.Run("a provider with local keys, token locations and forwarding is stored as written", func(t *testing.T) {
		entry := localProvider("okta", jwks)
		entry.ClockSkewSeconds = 15
		entry.Locations = []*api.JWTLocation{
			{Header: &api.JWTLocationHeader{Name: "Authorization", ValuePrefix: "Bearer "}},
			{QueryParam: &api.JWTLocationQueryParam{Name: "access_token"}},
			{Cookie: &api.JWTLocationCookie{Name: "session_jwt"}},
		}
		entry.Forwarding = &api.JWTForwardingConfig{HeaderName: "x-jwt-payload", PadForwardPayloadHeader: true}
		setProvider(t, c, entry)

		got := getProvider(t, c, "okta")
		require.Equal(t, "https://issuer.example.test/", got.Issuer)
		require.Equal(t, []string{"consul-test"}, got.Audiences)
		require.Equal(t, jwks, got.JSONWebKeySet.Local.JWKS)
		require.Equal(t, 15, got.ClockSkewSeconds)
		require.Len(t, got.Locations, 3)
		require.Equal(t, "Bearer ", got.Locations[0].Header.ValuePrefix)
		require.Equal(t, "access_token", got.Locations[1].QueryParam.Name)
		require.Equal(t, "session_jwt", got.Locations[2].Cookie.Name)
		require.Equal(t, "x-jwt-payload", got.Forwarding.HeaderName)
		require.True(t, got.Forwarding.PadForwardPayloadHeader)

		entries, _, err := c.ConfigEntries().List(api.JWTProvider, nil)
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.GetName())
		}
		require.Contains(t, names, "okta")
	})

	t.Run("a provider can fetch its keys from a remote JWKS endpoint", func(t *testing.T) {
		setProvider(t, c, &api.JWTProviderConfigEntry{
			Kind:   api.JWTProvider,
			Name:   "remote-keys",
			Issuer: "https://issuer.example.test/",
			JSONWebKeySet: &api.JSONWebKeySet{Remote: &api.RemoteJWKS{
				URI:                 "https://keys.example.test/.well-known/jwks.json",
				RequestTimeoutMs:    1500,
				CacheDuration:       10 * time.Minute,
				FetchAsynchronously: true,
			}},
		})

		remote := getProvider(t, c, "remote-keys").JSONWebKeySet.Remote
		require.Equal(t, "https://keys.example.test/.well-known/jwks.json", remote.URI)
		require.Equal(t, 1500, remote.RequestTimeoutMs)
		require.Equal(t, 10*time.Minute, remote.CacheDuration)
		require.True(t, remote.FetchAsynchronously)
	})

	t.Run("rotating the keys updates the stored key set", func(t *testing.T) {
		before := getProvider(t, c, "okta")
		rotated := encodeJWKS(t)
		require.NotEqual(t, before.JSONWebKeySet.Local.JWKS, rotated)

		update := *before
		update.JSONWebKeySet = &api.JSONWebKeySet{Local: &api.LocalJWKS{JWKS: rotated}}
		setProvider(t, c, &update)

		after := getProvider(t, c, "okta")
		require.Equal(t, rotated, after.JSONWebKeySet.Local.JWKS)
		require.Greater(t, after.ModifyIndex, before.ModifyIndex)
		require.Equal(t, before.CreateIndex, after.CreateIndex, "it is the same entry, updated")
		require.Equal(t, before.Issuer, after.Issuer, "the rest of the provider is unchanged")
	})

	t.Run("invalid providers are refused and not stored", func(t *testing.T) {
		withKeys := func(name string, keys *api.JSONWebKeySet) *api.JWTProviderConfigEntry {
			entry := localProvider(name, jwks)
			entry.JSONWebKeySet = keys
			return entry
		}
		withLocations := func(name string, locations ...*api.JWTLocation) *api.JWTProviderConfigEntry {
			entry := localProvider(name, jwks)
			entry.Locations = locations
			return entry
		}

		for name, tc := range map[string]struct {
			entry   *api.JWTProviderConfigEntry
			wantErr string
		}{
			"no key set":                     {withKeys("no-keys", nil), "JSONWebKeySet is required"},
			"an empty key set":               {withKeys("empty-keys", &api.JSONWebKeySet{}), "exactly one of Local or Remote"},
			"both local and remote keys":     {withKeys("both-keys", &api.JSONWebKeySet{Local: &api.LocalJWKS{JWKS: jwks}, Remote: &api.RemoteJWKS{URI: "https://keys.example.test/jwks"}}), "exactly one of Local or Remote"},
			"local keys that are not base64": {withKeys("bad-base64", &api.JSONWebKeySet{Local: &api.LocalJWKS{JWKS: "!!!"}}), "JWKS must be valid base64"},
			"local keys given twice":         {withKeys("two-local", &api.JSONWebKeySet{Local: &api.LocalJWKS{JWKS: jwks, Filename: "/etc/jwks.json"}}), "exactly one of String or filename"},
			"remote keys without a URI":      {withKeys("no-uri", &api.JSONWebKeySet{Remote: &api.RemoteJWKS{}}), "Remote JWKS URI is required"},
			"a location with nothing set":    {withLocations("empty-location", &api.JWTLocation{}), "exactly one of: JWT location header, query param or cookie"},
			"a location with two kinds": {withLocations("double-location", &api.JWTLocation{
				Header: &api.JWTLocationHeader{Name: "Authorization"}, Cookie: &api.JWTLocationCookie{Name: "jwt"},
			}), "exactly one of: JWT location header, query param or cookie"},
			"a header location without a name": {withLocations("nameless-header", &api.JWTLocation{Header: &api.JWTLocationHeader{}}), "header name must be specified"},
			"no name":                          {localProvider("", jwks), "Name is required"},
		} {
			ok, _, err := c.ConfigEntries().Set(tc.entry, nil)
			require.Error(t, err, name)
			require.False(t, ok, name)
			require.Contains(t, err.Error(), tc.wantErr, name)

			if tc.entry.Name != "" {
				_, _, err = c.ConfigEntries().Get(api.JWTProvider, tc.entry.Name, nil)
				require.Error(t, err, "a refused provider must not be stored: %s", name)
			}
		}
	})

	t.Run("an unreferenced provider can be deleted", func(t *testing.T) {
		_, err := c.ConfigEntries().Delete(api.JWTProvider, "remote-keys", nil)
		require.NoError(t, err)

		_, _, err = c.ConfigEntries().Get(api.JWTProvider, "remote-keys", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "404")
	})
}

// TestJWTProvider_IntentionReferences verifies the link between intentions and
// jwt-provider entries: an intention can only name providers that exist, and a
// provider that is in use cannot be deleted.
func TestJWTProvider_IntentionReferences(t *testing.T) {
	t.Parallel()

	c := newConfigClient(t)
	setProvider(t, c, localProvider("okta", encodeJWKS(t)))
	setProvider(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "api", Protocol: "http"})

	intention := func(jwt *api.IntentionJWTRequirement) *api.ServiceIntentionsConfigEntry {
		return &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "api",
			JWT:     jwt,
			Sources: []*api.SourceIntention{{Name: "web", Action: api.IntentionActionAllow}},
		}
	}

	t.Run("an intention cannot require a provider that does not exist", func(t *testing.T) {
		ok, _, err := c.ConfigEntries().Set(intention(&api.IntentionJWTRequirement{
			Providers: []*api.IntentionJWTProvider{{Name: "ghost"}},
		}), nil)

		require.Error(t, err)
		require.False(t, ok)
		require.Contains(t, err.Error(), "Referenced JWT Provider does not exist. Provider Name: ghost")
	})

	t.Run("an intention can require a provider and specific claims", func(t *testing.T) {
		setProvider(t, c, intention(&api.IntentionJWTRequirement{
			Providers: []*api.IntentionJWTProvider{{
				Name:         "okta",
				VerifyClaims: []*api.IntentionJWTClaimVerification{{Path: []string{"roles"}, Value: "admin"}},
			}},
		}))

		entry, _, err := c.ConfigEntries().Get(api.ServiceIntentions, "api", nil)
		require.NoError(t, err)
		jwt := entry.(*api.ServiceIntentionsConfigEntry).JWT
		require.Len(t, jwt.Providers, 1)
		require.Equal(t, "okta", jwt.Providers[0].Name)
		require.Equal(t, []string{"roles"}, jwt.Providers[0].VerifyClaims[0].Path)
		require.Equal(t, "admin", jwt.Providers[0].VerifyClaims[0].Value)
	})

	t.Run("a JWT requirement on a permission needs an HTTP match", func(t *testing.T) {
		entry := &api.ServiceIntentionsConfigEntry{
			Kind: api.ServiceIntentions,
			Name: "api",
			Sources: []*api.SourceIntention{{
				Name: "web",
				Permissions: []*api.IntentionPermission{{
					Action: api.IntentionActionAllow,
					JWT:    &api.IntentionJWTRequirement{Providers: []*api.IntentionJWTProvider{{Name: "okta"}}},
				}},
			}},
		}
		_, _, err := c.ConfigEntries().Set(entry, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "HTTP is required")

		entry.Sources[0].Permissions[0].HTTP = &api.IntentionHTTPPermission{PathPrefix: "/"}
		_, _, err = c.ConfigEntries().Set(entry, nil)
		require.NoError(t, err)

		// Restore the top-level requirement for the next steps.
		setProvider(t, c, intention(&api.IntentionJWTRequirement{Providers: []*api.IntentionJWTProvider{{Name: "okta"}}}))
	})

	t.Run("a provider that an intention uses cannot be deleted", func(t *testing.T) {
		_, err := c.ConfigEntries().Delete(api.JWTProvider, "okta", nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot delete jwt provider config entry referenced by an intention")
		getProvider(t, c, "okta")
	})

	t.Run("once nothing references it, the provider can be deleted", func(t *testing.T) {
		_, err := c.ConfigEntries().Delete(api.ServiceIntentions, "api", nil)
		require.NoError(t, err)

		_, err = c.ConfigEntries().Delete(api.JWTProvider, "okta", nil)
		require.NoError(t, err)
	})
}
