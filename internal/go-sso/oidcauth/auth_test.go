// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidcauth

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/go-hclog"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
)

// mockConfig is a minimal valid config for testing.
func mockConfig(typ string, t *testing.T) *Config {
	t.Helper()

	srv := oidcauthtest.Start(t)
	srv.SetClientCreds("abc", "def")
	cfg := &Config{
		Type: typ,
	}
	if typ == TypeJWT {
		cfg.JWKSURL = srv.Addr() + "/certs"
		cfg.JWKSCACert = srv.CACert()
	}
	if typ == TypeOIDC {
		cfg.OIDCDiscoveryURL = srv.Addr()
		cfg.OIDCClientID = "abc"
		cfg.OIDCClientSecret = "def"
		cfg.AllowedRedirectURIs = []string{"https://redirect"}
		cfg.OIDCDiscoveryCACert = srv.CACert()
	}
	return cfg
}

const testPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEEVs/o5+uQbTjL3chynL4wXgUg2R9
q9UU8I5mEovUf86QZ7kOBIjJwqnzD1omageEHWwHdBO6B+dFabmdT9POxg==
-----END PUBLIC KEY-----`

func TestAuthenticator_JWTGroup(t *testing.T) {
	t.Run("JWTType static keys", func(t *testing.T) {
		cfg := mockConfig(TypeJWT, t)
		cfg.JWTValidationPubKeys = []string{testPublicKeyPEM}
		cfg.JWKSURL = ""
		cfg.JWKSCACert = ""
		logger := hclog.NewNullLogger()
		auth, err := New(cfg, logger)
		assert.NoError(t, err)
		assert.NotNil(t, auth)
		assert.Equal(t, cfg, auth.config)
		assert.NotEmpty(t, auth.parsedJWTPubKeys)
	})

	t.Run("JWTType JWKS", func(t *testing.T) {
		cfg := mockConfig(TypeJWT, t)
		logger := hclog.NewNullLogger()
		auth, err := New(cfg, logger)
		assert.NoError(t, err)
		assert.NotNil(t, auth)
		assert.Equal(t, cfg, auth.config)
	})

	t.Run("JWTType failure", func(t *testing.T) {
		cfg := mockConfig(TypeJWT, t)
		cfg.OIDCClientID = "abc"
		logger := hclog.NewNullLogger()
		_, err := New(cfg, logger)
		assert.Error(t, err)
		requireErrorContains(t, err, "'OIDCClientID' must not be set for type")
	})

	t.Run("Stop", func(t *testing.T) {
		cfg := mockConfig(TypeJWT, t)
		logger := hclog.NewNullLogger()
		auth, err := New(cfg, logger)
		assert.NoError(t, err)
		assert.NotNil(t, auth.backgroundCtxCancel)
		auth.Stop()
		assert.Nil(t, auth.backgroundCtxCancel)
	})

	t.Run("BackgroundContextCancel", func(t *testing.T) {
		cfg := mockConfig(TypeJWT, t)
		logger := hclog.NewNullLogger()
		auth, err := New(cfg, logger)
		assert.NoError(t, err)
		done := make(chan struct{})
		go func() {
			<-auth.backgroundCtx.Done()
			close(done)
		}()
		auth.Stop()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("backgroundCtx was not cancelled")
		}
	})
}

func TestAuthenticator_OIDCGroup(t *testing.T) {
	t.Run("OIDCType", func(t *testing.T) {
		cfg := mockConfig(TypeOIDC, t)
		logger := hclog.NewNullLogger()
		auth, err := New(cfg, logger)
		assert.NoError(t, err)
		assert.NotNil(t, auth.capProvider)
		assert.NotNil(t, auth.oidcStates)
	})

	t.Run("OIDCDiscovery", func(t *testing.T) {
		srv := oidcauthtest.Start(t)
		srv.SetClientCreds("abc", "def")
		cfg := mockConfig(TypeJWT, t)
		cfg.JWKSURL = ""
		cfg.JWKSCACert = ""
		cfg.OIDCDiscoveryURL = srv.Addr()
		cfg.OIDCDiscoveryCACert = srv.CACert()

		logger := hclog.NewNullLogger()
		auth, err := New(cfg, logger)
		assert.NoError(t, err)
		assert.NotNil(t, auth)
		assert.NotNil(t, auth.provider)
		assert.NotNil(t, auth.httpClient)
	})

	t.Run("OIDCStatesCache", func(t *testing.T) {
		cfg := mockConfig(TypeOIDC, t)
		logger := hclog.NewNullLogger()
		auth, err := New(cfg, logger)
		assert.NoError(t, err)
		assert.NotNil(t, auth.oidcStates)
		auth.oidcStates.Set("state", "value", cache.DefaultExpiration)
		val, found := auth.oidcStates.Get("state")
		assert.True(t, found)
		assert.Equal(t, "value", val)
	})
}

func TestAuthenticator_OIDCFlow_Failure(t *testing.T) {
	t.Run("InvalidCACert", func(t *testing.T) {
		cfg := mockConfig(TypeOIDC, t)
		cfg.OIDCDiscoveryCACert = "invalid cert data"

		logger := hclog.NewNullLogger()
		_, err := New(cfg, logger)

		assert.Error(t, err)
		requireErrorContains(t, err, "could not parse CA PEM value successfully")
	})

	t.Run("ProviderConfig_error", func(t *testing.T) {
		cfg := mockConfig(TypeOIDC, t)
		cfg.OIDCDiscoveryURL = "::invalid-url::"

		logger := hclog.NewNullLogger()
		_, err := New(cfg, logger)

		assert.Error(t, err)
		// Should match the actual error pattern from the code
		requireErrorContains(t, err, "error checking OIDCDiscoveryURL")
	})
}
