package oidcauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"gopkg.in/square/go-jose.v2/jwt"
)

const claimDefaultLeeway = 150

// ClaimsFromJWT is unrelated to the OIDC authorization code workflow. This
// allows for a JWT to be directly validated and decoded into a set of claims.
//
// Requires the authenticator's config type be set to 'jwt'.
func (a *Authenticator) ClaimsFromJWT(ctx context.Context, jwt string) (*Claims, error) {
	if a.config.authType() == authOIDCFlow {
		return nil, fmt.Errorf("ClaimsFromJWT is incompatible with type %q", TypeOIDC)
	}
	if jwt == "" {
		return nil, errors.New("missing jwt")
	}

	// Here is where things diverge. If it is using OIDC Discovery, validate that way;
	// otherwise validate against the locally configured or JWKS keys. Once things are
	// validated, we re-unify the request path when evaluating the claims.
	var (
		allClaims map[string]interface{}
		err       error
	)
	switch a.config.authType() {
	case authStaticKeys, authJWKS:
		allClaims, err = a.verifyVanillaJWT(ctx, jwt)
		if err != nil {
			return nil, err
		}

	case authOIDCDiscovery:
		allClaims, err = a.verifyOIDCToken(ctx, jwt)
		if err != nil {
			return nil, err
		}

	default:
		return nil, errors.New("unhandled case during login")
	}

	c, err := a.extractClaims(allClaims)
	if err != nil {
		return nil, err
	}

	if a.config.VerboseOIDCLogging && a.logger != nil {
		a.logger.Debug("OIDC provider response", "extracted_claims", c)
	}

	return c, nil
}

func (a *Authenticator) verifyVanillaJWT(ctx context.Context, loginToken string) (map[string]interface{}, error) {
	var (
		allClaims = map[string]interface{}{}
		claims    = jwt.Claims{}
	)
	// TODO(sso): handle JWTSupportedAlgs
	switch a.config.authType() {
	case authJWKS:
		// Verify signature (and only signature... other elements are checked later)
		payload, err := a.keySet.VerifySignature(ctx, loginToken)
		if err != nil {
			return nil, fmt.Errorf("error verifying token: %v", err)
		}

		// Unmarshal payload into two copies: public claims for library verification, and a set
		// of all received claims.
		if err := json.Unmarshal(payload, &claims); err != nil {
			return nil, fmt.Errorf("failed to unmarshal claims: %v", err)
		}
		if err := json.Unmarshal(payload, &allClaims); err != nil {
			return nil, fmt.Errorf("failed to unmarshal claims: %v", err)
		}
	case authStaticKeys:
		parsedJWT, err := jwt.ParseSigned(loginToken)
		if err != nil {
			return nil, fmt.Errorf("error parsing token: %v", err)
		}

		var valid bool
		for _, key := range a.parsedJWTPubKeys {
			if err := parsedJWT.Claims(key, &claims, &allClaims); err == nil {
				valid = true
				break
			}
		}
		if !valid {
			return nil, errors.New("no known key successfully validated the token signature")
		}
	default:
		return nil, fmt.Errorf("unsupported auth type for this verifyVanillaJWT: %d", a.config.authType())
	}

	// We require notbefore or expiry; if only one is provided, we allow 5 minutes of leeway by default.
	// Configurable by ExpirationLeeway and NotBeforeLeeway
	if claims.IssuedAt == nil {
		claims.IssuedAt = new(jwt.NumericDate)
	}
	if claims.Expiry == nil {
		claims.Expiry = new(jwt.NumericDate)
	}
	if claims.NotBefore == nil {
		claims.NotBefore = new(jwt.NumericDate)
	}
	if *claims.IssuedAt == 0 && *claims.Expiry == 0 && *claims.NotBefore == 0 {
		return nil, errors.New("no issue time, notbefore, or expiration time encoded in token")
	}

	if *claims.Expiry == 0 {
		latestStart := *claims.IssuedAt
		if *claims.NotBefore > *claims.IssuedAt {
			latestStart = *claims.NotBefore
		}
		leeway := a.config.ExpirationLeeway.Seconds()
		if a.config.ExpirationLeeway.Seconds() < 0 {
			leeway = 0
		} else if a.config.ExpirationLeeway.Seconds() == 0 {
			leeway = claimDefaultLeeway
		}
		*claims.Expiry = jwt.NumericDate(int64(latestStart) + int64(leeway))
	}

	if *claims.NotBefore == 0 {
		if *claims.IssuedAt != 0 {
			*claims.NotBefore = *claims.IssuedAt
		} else {
			leeway := a.config.NotBeforeLeeway.Seconds()
			if a.config.NotBeforeLeeway.Seconds() < 0 {
				leeway = 0
			} else if a.config.NotBeforeLeeway.Seconds() == 0 {
				leeway = claimDefaultLeeway
			}
			*claims.NotBefore = jwt.NumericDate(int64(*claims.Expiry) - int64(leeway))
		}
	}

	expected := jwt.Expected{
		Issuer: a.config.BoundIssuer,
		// Subject: a.config.BoundSubject,
		Time: time.Now(),
	}

	cksLeeway := a.config.ClockSkewLeeway
	if a.config.ClockSkewLeeway.Seconds() < 0 {
		cksLeeway = 0
	} else if a.config.ClockSkewLeeway.Seconds() == 0 {
		cksLeeway = jwt.DefaultLeeway
	}

	if err := claims.ValidateWithLeeway(expected, cksLeeway); err != nil {
		return nil, fmt.Errorf("error validating claims: %v", err)
	}

	if err := validateAudience(a.config.BoundAudiences, claims.Audience, true); err != nil {
		return nil, fmt.Errorf("error validating claims: %v", err)
	}

	return allClaims, nil
}

// parsePublicKeyPEM is used to parse RSA, ECDSA, and Ed25519 public keys from PEMs
//
// Extracted from "github.com/hashicorp/vault/sdk/helper/certutil"
//
// go-sso added support for ed25519 (EdDSA)
func parsePublicKeyPEM(data []byte) (interface{}, error) {
	block, _ := pem.Decode(data)
	if block != nil {
		var rawKey interface{}
		var err error
		if rawKey, err = x509.ParsePKIXPublicKey(block.Bytes); err != nil {
			if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
				rawKey = cert.PublicKey
			} else {
				return nil, err
			}
		}

		if rsaPublicKey, ok := rawKey.(*rsa.PublicKey); ok {
			return rsaPublicKey, nil
		}
		if ecPublicKey, ok := rawKey.(*ecdsa.PublicKey); ok {
			return ecPublicKey, nil
		}
		if edPublicKey, ok := rawKey.(ed25519.PublicKey); ok {
			return edPublicKey, nil
		}
	}

	return nil, errors.New("data does not contain any valid RSA, ECDSA, or ED25519 public keys")
}
