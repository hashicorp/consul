package oidcauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
)

const (
	// TypeOIDC is the config type to specify if the OIDC authorization code
	// workflow is desired. The Authenticator methods GetAuthCodeURL and
	// ClaimsFromAuthCode are activated with the type.
	TypeOIDC = "oidc"

	// TypeJWT is the config type to specify if simple JWT decoding (via static
	// keys, JWKS, and OIDC discovery) is desired.  The Authenticator method
	// ClaimsFromJWT is activated with this type.
	TypeJWT = "jwt"
)

// Config is the collection of all settings that pertain to doing OIDC-based
// authentication and direct JWT-based authentication processes.
type Config struct {
	// Type defines which kind of authentication will be happening, OIDC-based
	// or JWT-based.  Allowed values are either 'oidc' or 'jwt'.
	//
	// Defaults to 'oidc' if unset.
	Type string

	// -------
	// common for type=oidc and type=jwt
	// -------

	// JWTSupportedAlgs is a list of supported signing algorithms. Defaults to
	// RS256.
	JWTSupportedAlgs []string

	// Comma-separated list of 'aud' claims that are valid for login; any match
	// is sufficient
	// TODO(sso): actually just send these down as string claims?
	BoundAudiences []string

	// Mappings of claims (key) that will be copied to a metadata field
	// (value). Use this if the claim you are capturing is singular (such as an
	// attribute).
	//
	// When mapped, the values can be any of a number, string, or boolean and
	// will all be stringified when returned.
	ClaimMappings map[string]string

	// Mappings of claims (key) that will be copied to a metadata field
	// (value). Use this if the claim you are capturing is list-like (such as
	// groups).
	//
	// When mapped, the values in each list can be any of a number, string, or
	// boolean and will all be stringified when returned.
	ListClaimMappings map[string]string

	// OIDCDiscoveryURL is the OIDC Discovery URL, without any .well-known
	// component (base path).  Cannot be used with "JWKSURL" or
	// "JWTValidationPubKeys".
	OIDCDiscoveryURL string

	// OIDCDiscoveryCACert is the CA certificate or chain of certificates, in
	// PEM format, to use to validate connections to the OIDC Discovery URL. If
	// not set, system certificates are used.
	OIDCDiscoveryCACert string

	// -------
	// just for type=oidc
	// -------

	// OIDCClientID is the OAuth Client ID configured with your OIDC provider.
	//
	// Valid only if Type=oidc
	OIDCClientID string

	// The OAuth Client Secret configured with your OIDC provider.
	//
	// Valid only if Type=oidc
	OIDCClientSecret string

	// Comma-separated list of OIDC scopes
	//
	// Valid only if Type=oidc
	OIDCScopes []string

	// Space-separated list of OIDC Authorization Context Class Reference values
	//
	// Valid only if Type=oidc
	OIDCACRValues []string

	// Comma-separated list of allowed values for redirect_uri
	//
	// Valid only if Type=oidc
	AllowedRedirectURIs []string

	// Log received OIDC tokens and claims when debug-level logging is active.
	// Not recommended in production since sensitive information may be present
	// in OIDC responses.
	//
	// Valid only if Type=oidc
	VerboseOIDCLogging bool

	// -------
	// just for type=jwt
	// -------

	// JWKSURL is the JWKS URL to use to authenticate signatures. Cannot be
	// used with "OIDCDiscoveryURL" or "JWTValidationPubKeys".
	//
	// Valid only if Type=jwt
	JWKSURL string

	// JWKSCACert is the CA certificate or chain of certificates, in PEM
	// format, to use to validate connections to the JWKS URL. If not set,
	// system certificates are used.
	//
	// Valid only if Type=jwt
	JWKSCACert string

	// JWTValidationPubKeys is a list of PEM-encoded public keys to use to
	// authenticate signatures locally. Cannot be used with "JWKSURL" or
	// "OIDCDiscoveryURL".
	//
	// Valid only if Type=jwt
	JWTValidationPubKeys []string

	// BoundIssuer is the value against which to match the 'iss' claim in a
	// JWT.  Optional.
	//
	// Valid only if Type=jwt
	BoundIssuer string

	// Duration in seconds of leeway when validating expiration of
	// a token to account for clock skew.
	//
	// Defaults to 150 (2.5 minutes) if set to 0 and can be disabled if set to -1.`,
	//
	// Valid only if Type=jwt
	ExpirationLeeway time.Duration

	// Duration in seconds of leeway when validating not before values of a
	// token to account for clock skew.
	//
	// Defaults to 150 (2.5 minutes) if set to 0 and can be disabled if set to
	// -1.`,
	//
	// Valid only if Type=jwt
	NotBeforeLeeway time.Duration

	// Duration in seconds of leeway when validating all claims to account for
	// clock skew.
	//
	// Defaults to 60 (1 minute) if set to 0 and can be disabled if set to
	// -1.`,
	//
	// Valid only if Type=jwt
	ClockSkewLeeway time.Duration
}

// Validate returns an error if the config is not valid.
func (c *Config) Validate() error {
	validateCtx, validateCtxCancel := context.WithCancel(context.Background())
	defer validateCtxCancel()

	switch c.Type {
	case TypeOIDC, "":
		// required
		switch {
		case c.OIDCDiscoveryURL == "":
			return fmt.Errorf("'OIDCDiscoveryURL' must be set for type %q", c.Type)
		case c.OIDCClientID == "":
			return fmt.Errorf("'OIDCClientID' must be set for type %q", c.Type)
		case c.OIDCClientSecret == "":
			return fmt.Errorf("'OIDCClientSecret' must be set for type %q", c.Type)
		case len(c.AllowedRedirectURIs) == 0:
			return fmt.Errorf("'AllowedRedirectURIs' must be set for type %q", c.Type)
		}

		// not allowed
		switch {
		case c.JWKSURL != "":
			return fmt.Errorf("'JWKSURL' must not be set for type %q", c.Type)
		case c.JWKSCACert != "":
			return fmt.Errorf("'JWKSCACert' must not be set for type %q", c.Type)
		case len(c.JWTValidationPubKeys) != 0:
			return fmt.Errorf("'JWTValidationPubKeys' must not be set for type %q", c.Type)
		case c.BoundIssuer != "":
			return fmt.Errorf("'BoundIssuer' must not be set for type %q", c.Type)
		case c.ExpirationLeeway != 0:
			return fmt.Errorf("'ExpirationLeeway' must not be set for type %q", c.Type)
		case c.NotBeforeLeeway != 0:
			return fmt.Errorf("'NotBeforeLeeway' must not be set for type %q", c.Type)
		case c.ClockSkewLeeway != 0:
			return fmt.Errorf("'ClockSkewLeeway' must not be set for type %q", c.Type)
		}

		var bad []string
		for _, allowed := range c.AllowedRedirectURIs {
			if _, err := url.Parse(allowed); err != nil {
				bad = append(bad, allowed)
			}
		}
		if len(bad) > 0 {
			return fmt.Errorf("Invalid AllowedRedirectURIs provided: %v", bad)
		}

	case TypeJWT:
		// not allowed
		switch {
		case c.OIDCClientID != "":
			return fmt.Errorf("'OIDCClientID' must not be set for type %q", c.Type)
		case c.OIDCClientSecret != "":
			return fmt.Errorf("'OIDCClientSecret' must not be set for type %q", c.Type)
		case len(c.OIDCScopes) != 0:
			return fmt.Errorf("'OIDCScopes' must not be set for type %q", c.Type)
		case len(c.OIDCACRValues) != 0:
			return fmt.Errorf("'OIDCACRValues' must not be set for type %q", c.Type)
		case len(c.AllowedRedirectURIs) != 0:
			return fmt.Errorf("'AllowedRedirectURIs' must not be set for type %q", c.Type)
		case c.VerboseOIDCLogging:
			return fmt.Errorf("'VerboseOIDCLogging' must not be set for type %q", c.Type)
		}

		methodCount := 0
		if c.OIDCDiscoveryURL != "" {
			methodCount++
		}
		if len(c.JWTValidationPubKeys) != 0 {
			methodCount++
		}
		if c.JWKSURL != "" {
			methodCount++
		}

		if methodCount != 1 {
			return fmt.Errorf("exactly one of 'JWTValidationPubKeys', 'JWKSURL', or 'OIDCDiscoveryURL' must be set for type %q", c.Type)
		}

		if c.JWKSURL != "" {
			httpClient, err := createHTTPClient(c.JWKSCACert)
			if err != nil {
				return fmt.Errorf("error checking JWKSCACert: %v", err)
			}

			ctx := contextWithHttpClient(validateCtx, httpClient)
			keyset := oidc.NewRemoteKeySet(ctx, c.JWKSURL)

			// Try to verify a correctly formatted JWT. The signature will fail
			// to match, but other errors with fetching the remote keyset
			// should be reported.
			_, err = keyset.VerifySignature(ctx, testJWT)
			if err == nil {
				err = errors.New("unexpected verification of JWT")
			}

			if !strings.Contains(err.Error(), "failed to verify id token signature") {
				return fmt.Errorf("error checking JWKSURL: %v", err)
			}
		} else if c.JWKSCACert != "" {
			return fmt.Errorf("'JWKSCACert' should not be set unless 'JWKSURL' is set")
		}

		if len(c.JWTValidationPubKeys) != 0 {
			for i, v := range c.JWTValidationPubKeys {
				if _, err := parsePublicKeyPEM([]byte(v)); err != nil {
					return fmt.Errorf("error parsing public key JWTValidationPubKeys[%d]: %v", i, err)
				}
			}
		}

	default:
		return fmt.Errorf("authenticator type should be %q or %q", TypeOIDC, TypeJWT)
	}

	if c.OIDCDiscoveryURL != "" {
		httpClient, err := createHTTPClient(c.OIDCDiscoveryCACert)
		if err != nil {
			return fmt.Errorf("error checking OIDCDiscoveryCACert: %v", err)
		}

		ctx := contextWithHttpClient(validateCtx, httpClient)
		if _, err := oidc.NewProvider(ctx, c.OIDCDiscoveryURL); err != nil {
			return fmt.Errorf("error checking OIDCDiscoveryURL: %v", err)
		}
	} else if c.OIDCDiscoveryCACert != "" {
		return fmt.Errorf("'OIDCDiscoveryCACert' should not be set unless 'OIDCDiscoveryURL' is set")
	}

	for _, a := range c.JWTSupportedAlgs {
		switch a {
		case oidc.RS256, oidc.RS384, oidc.RS512,
			oidc.ES256, oidc.ES384, oidc.ES512,
			oidc.PS256, oidc.PS384, oidc.PS512:
		default:
			return fmt.Errorf("Invalid supported algorithm: %s", a)
		}
	}

	if len(c.ClaimMappings) > 0 {
		targets := make(map[string]bool)
		for _, mappedKey := range c.ClaimMappings {
			if targets[mappedKey] {
				return fmt.Errorf("ClaimMappings contains multiple mappings for key %q", mappedKey)
			}
			targets[mappedKey] = true
		}
	}

	if len(c.ListClaimMappings) > 0 {
		targets := make(map[string]bool)
		for _, mappedKey := range c.ListClaimMappings {
			if targets[mappedKey] {
				return fmt.Errorf("ListClaimMappings contains multiple mappings for key %q", mappedKey)
			}
			targets[mappedKey] = true
		}
	}

	return nil
}

const (
	authUnconfigured = iota
	authStaticKeys
	authJWKS
	authOIDCDiscovery
	authOIDCFlow
)

// authType classifies the authorization type/flow based on config parameters.
// It is only valid to invoke if Validate() returns a nil error.
func (c *Config) authType() int {
	switch {
	case len(c.JWTValidationPubKeys) > 0:
		return authStaticKeys
	case c.JWKSURL != "":
		return authJWKS
	case c.OIDCDiscoveryURL != "":
		if c.OIDCClientID != "" && c.OIDCClientSecret != "" {
			return authOIDCFlow
		}
		return authOIDCDiscovery
	default:
		return authUnconfigured
	}
}

const testJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.Hf3E3iCHzqC5QIQ0nCqS1kw78IiQTRVzsLTuKoDIpdk"
