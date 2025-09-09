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
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	capOidc "github.com/hashicorp/cap/oidc"
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

	// provider is the coreos/go-oidc provider used for JWT validation
	provider *oidc.Provider

	// capProvider is the HashiCorp CAP library provider used for OIDC flows
	// with support for private key JWT client authentication
	capProvider *capOidc.Provider
	keySet      oidc.KeySet

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

	var err error
	if c.Type == TypeOIDC {
		a.oidcStates = cache.New(oidcStateTimeout, oidcStateCleanupInterval)
	}

	switch c.authType() {
	case authOIDCFlow:
		var supported []capOidc.Alg
		if len(a.config.JWTSupportedAlgs) == 0 {
			// Default to RS256 if nothing is specified.
			supported = []capOidc.Alg{capOidc.RS256}
		} else {
			for _, alg := range a.config.JWTSupportedAlgs {
				supported = append(supported, capOidc.Alg(alg))
			}
		}
		// Use CAP's OIDC provider to leverage its built-in support for
		providerConfig, err := capOidc.NewConfig(
			a.config.OIDCDiscoveryURL,
			a.config.OIDCClientID,
			capOidc.ClientSecret(a.config.OIDCClientSecret),
			supported,
			a.config.AllowedRedirectURIs,
			capOidc.WithAudiences(a.config.BoundAudiences...),
			capOidc.WithProviderCA(a.config.OIDCDiscoveryCACert),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating provider config: %v", err)
		}

		provider, error := capOidc.NewProvider(providerConfig)
		if error != nil {
			return nil, fmt.Errorf("error creating provider: %v", error)
		}
		a.capProvider = provider
	case authOIDCDiscovery:
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

// String implements the fmt.Stringer interface for detailed logging of Authenticator
func (a *Authenticator) String() string {
	if a == nil {
		return "Authenticator<nil>"
	}

	var details strings.Builder
	details.WriteString("Authenticator{\n")

	// Config information
	if a.config != nil {
		details.WriteString(fmt.Sprintf("  config: {\n"))
		details.WriteString(fmt.Sprintf("    Type: %q\n", a.config.Type))
		details.WriteString(fmt.Sprintf("    OIDCDiscoveryURL: %q\n", a.config.OIDCDiscoveryURL))
		details.WriteString(fmt.Sprintf("    OIDCClientID: %q\n", a.config.OIDCClientID))
		details.WriteString(fmt.Sprintf("    HasClientSecret: %v\n", a.config.OIDCClientSecret != ""))
		details.WriteString(fmt.Sprintf("    AllowedRedirectURIs: %v\n", a.config.AllowedRedirectURIs))
		details.WriteString(fmt.Sprintf("    OIDCScopes: %v\n", a.config.OIDCScopes))
		details.WriteString(fmt.Sprintf("    BoundAudiences: %v\n", a.config.BoundAudiences))
		details.WriteString(fmt.Sprintf("    JWTValidationPubKeys: %d keys\n", len(a.config.JWTValidationPubKeys)))
		details.WriteString(fmt.Sprintf("    JWTSupportedAlgs: %v\n", a.config.JWTSupportedAlgs))
		details.WriteString(fmt.Sprintf("    ClaimMappings: %v\n", a.config.ClaimMappings))
		details.WriteString(fmt.Sprintf("    ListClaimMappings: %v\n", a.config.ListClaimMappings))
		details.WriteString(fmt.Sprintf("    OIDCDisablePKCE: %v\n", a.config.OIDCClientUsePKCE))
		details.WriteString(fmt.Sprintf("    VerboseOIDCLogging: %v\n", a.config.VerboseOIDCLogging))

		// Client Assertion details (if available)
		if a.config.OIDCClientAssertion != nil {
			details.WriteString("    OIDCClientAssertion: {\n")
			details.WriteString(fmt.Sprintf("      HasPrivateKey: %v\n",
				a.config.OIDCClientAssertion.PrivateKey != nil &&
					a.config.OIDCClientAssertion.PrivateKey.PemKey != ""))
			details.WriteString(fmt.Sprintf("      Audience: %v\n", a.config.OIDCClientAssertion.Audience))
			details.WriteString(fmt.Sprintf("      KeyAlgorithm: %q\n", a.config.OIDCClientAssertion.KeyAlgorithm))
			details.WriteString("    }\n")
		} else {
			details.WriteString("    OIDCClientAssertion: <nil>\n")
		}
		details.WriteString("  },\n")
	} else {
		details.WriteString("  config: <nil>,\n")
	}

	// Provider information
	details.WriteString(fmt.Sprintf("  provider: %v,\n", a.provider != nil))
	details.WriteString(fmt.Sprintf("  capProvider: %v,\n", a.capProvider != nil))
	details.WriteString(fmt.Sprintf("  keySet: %v,\n", a.keySet != nil))
	details.WriteString(fmt.Sprintf("  httpClient: %v,\n", a.httpClient != nil))

	// OIDC state information
	stateCount := 0
	if a.oidcStates != nil {
		stateCount = a.oidcStates.ItemCount()
	}
	details.WriteString(fmt.Sprintf("  oidcStates: %d active states,\n", stateCount))

	// Background context
	details.WriteString(fmt.Sprintf("  backgroundCtx: %v,\n", a.backgroundCtx != nil))
	details.WriteString(fmt.Sprintf("  backgroundCtxCancel: %v,\n", a.backgroundCtxCancel != nil))

	// JWT info
	details.WriteString(fmt.Sprintf("  parsedJWTPubKeys: %d keys\n", len(a.parsedJWTPubKeys)))
	details.WriteString("}")

	return details.String()
}
