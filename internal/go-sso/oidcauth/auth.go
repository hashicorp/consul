// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// package oidcauth bundles up an opinionated approach to authentication using
// both the OIDC authorization code workflow and simple JWT decoding (via
// static keys, JWKS, and OIDC discovery).
//
// NOTE: This was roughly forked from hashicorp/vault-plugin-auth-jwt
// originally at commit 825c85535e3832d254a74253a8e9ae105357778b with later
// backports of behavior in 0e93b06cecb0477d6ee004e44b04832d110096cf
package oidcauth

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/coreos/go-oidc"
	"github.com/hashicorp/go-hclog"
	"github.com/patrickmn/go-cache"
)

// Claims represents a set of claims or assertions computed about a given
// authentication exchange.
type Claims struct {
	// Values is a set of key/value string claims about the authentication
	// exchange.
	Values map[string]string

	// Lists is a set of key/value string list claims about the authentication
	// exchange.
	Lists map[string][]string
}

// Authenticator allows for extracting a set of claims from either an OIDC
// authorization code exchange or a bare JWT.
type Authenticator struct {
	config *Config
	logger hclog.Logger

	// parsedJWTPubKeys is the parsed form of config.JWTValidationPubKeys
	parsedJWTPubKeys []interface{}
	provider         *oidc.Provider
	keySet           oidc.KeySet

	// httpClient should be configured with all relevant root CA certs and be
	// reused for all OIDC or JWKS operations. This will be nil for the static
	// keys JWT configuration.
	httpClient *http.Client

	l          sync.Mutex
	oidcStates *cache.Cache

	// backgroundCtx is a cancellable context primarily meant to be used for
	// things that may spawn background goroutines and are not tied to a
	// request/response lifecycle. Use backgroundCtxCancel to cancel this.
	backgroundCtx       context.Context
	backgroundCtxCancel context.CancelFunc
}

// New creates an authenticator suitable for use with either an OIDC
// authorization code workflow or a bare JWT workflow depending upon the value
// of the config Type.
func New(c *Config, logger hclog.Logger) (*Authenticator, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	var parsedJWTPubKeys []interface{}
	if c.Type == TypeJWT {
		for _, v := range c.JWTValidationPubKeys {
			key, err := parsePublicKeyPEM([]byte(v))
			if err != nil {
				// This shouldn't happen as the keys are already validated in Validate().
				return nil, fmt.Errorf("error parsing public key: %v", err)
			}
			parsedJWTPubKeys = append(parsedJWTPubKeys, key)
		}
	}

	a := &Authenticator{
		config:           c,
		logger:           logger,
		parsedJWTPubKeys: parsedJWTPubKeys,
	}
	a.backgroundCtx, a.backgroundCtxCancel = context.WithCancel(context.Background())

	if c.Type == TypeOIDC {
		a.oidcStates = cache.New(oidcStateTimeout, oidcStateCleanupInterval)
	}

	var err error
	switch c.authType() {
	case authOIDCDiscovery, authOIDCFlow:
		a.httpClient, err = createHTTPClient(a.config.OIDCDiscoveryCACert)
		if err != nil {
			return nil, fmt.Errorf("error parsing OIDCDiscoveryCACert: %v", err)
		}

		provider, err := oidc.NewProvider(
			contextWithHttpClient(a.backgroundCtx, a.httpClient),
			a.config.OIDCDiscoveryURL,
		)
		if err != nil {
			return nil, fmt.Errorf("error creating provider: %v", err)
		}
		a.provider = provider
	case authJWKS:
		a.httpClient, err = createHTTPClient(a.config.JWKSCACert)
		if err != nil {
			return nil, fmt.Errorf("error parsing JWKSCACert: %v", err)
		}

		a.keySet = oidc.NewRemoteKeySet(
			contextWithHttpClient(a.backgroundCtx, a.httpClient),
			a.config.JWKSURL,
		)
	}

	return a, nil
}

// Stop stops any background goroutines and does cleanup.
func (a *Authenticator) Stop() {
	a.l.Lock()
	defer a.l.Unlock()
	if a.backgroundCtxCancel != nil {
		a.backgroundCtxCancel()
		a.backgroundCtxCancel = nil
	}
}
