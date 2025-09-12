// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidcauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/cap/oidc"
	cass "github.com/hashicorp/cap/oidc/clientassertion"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/go-uuid"
)

var (
	oidcStateTimeout         = 10 * time.Minute
	oidcStateCleanupInterval = 1 * time.Minute
)

// GetAuthCodeURL is the first part of the OIDC authorization code workflow.
// The statePayload field is stored in the Authenticator instance keyed by the
// "state" key so it can be returned during a future call to
// ClaimsFromAuthCode.
//
// Requires the authenticator's config type be set to 'oidc'.
func (a *Authenticator) GetAuthCodeURL(ctx context.Context, redirectURI string, statePayload interface{}) (string, error) {
	if a.config.authType() != authOIDCFlow {
		return "", fmt.Errorf("GetAuthCodeURL is incompatible with type %q", TypeJWT)
	}
	if redirectURI == "" {
		return "", errors.New("missing redirect_uri")
	}

	if !validRedirect(redirectURI, a.config.AllowedRedirectURIs) {
		return "", fmt.Errorf("unauthorized redirect_uri: %s", redirectURI)
	}

	// Use HashiCorp CAP provider which supports advanced OIDC features
	// including private key JWT client authentication configured during initialization
	provider := a.capProvider
	payload := statePayload

	// Generate a secure state and nonce for the OIDC request
	// The request object is stored for later use during token exchange
	request, error := a.createOIDCState(redirectURI, payload)
	if error != nil {
		return "", fmt.Errorf("Error generating OAuth state: %v", error)
	}

	authURL, err := provider.AuthURL(ctx, request)
	if err != nil {
		return "", fmt.Errorf("Error while generating AuthURL %q", err)
	}

	return authURL, nil
}

// ClaimsFromAuthCode is the second part of the OIDC authorization code
// workflow. The interface{} return value is the statePayload previously passed
// via GetAuthCodeURL.
//
// The error may be of type *ProviderLoginFailedError or
// *TokenVerificationFailedError which can be detected via errors.As().
//
// Requires the authenticator's config type be set to 'oidc'.
func (a *Authenticator) ClaimsFromAuthCode(ctx context.Context, stateParam, code string) (*Claims, interface{}, error) {
	if a.config.authType() != authOIDCFlow {
		return nil, nil, fmt.Errorf("ClaimsFromAuthCode is incompatible with type %q", TypeJWT)
	}

	// TODO(sso): this could be because we ACTUALLY are getting OIDC error responses and
	// should handle them elsewhere!
	if code == "" {
		return nil, nil, &ProviderLoginFailedError{
			Err: fmt.Errorf("OAuth code parameter not provided"),
		}
	}

	state := a.verifyOIDCState(stateParam)
	if state == nil {
		return nil, nil, &ProviderLoginFailedError{
			Err: fmt.Errorf("Expired or missing OAuth state."),
		}
	}

	// Use the stored request object from the initial authorization request
	if state.request == nil {
		a.logger.Error("Request object not found in state", "stateParam", stateParam)
		return nil, nil, &ProviderLoginFailedError{
			Err: fmt.Errorf("missing request object in OAuth state"),
		}
	}

	// Use HashiCorp CAP provider for token exchange
	// This provider supports private key JWT client authentication if configured
	provider := a.capProvider

	tokens, err := provider.Exchange(ctx, state.request, stateParam, code)
	if err != nil {
		return nil, nil, &ProviderLoginFailedError{
			Err: fmt.Errorf("Error exchanging oidc code: %w", err),
		}
	}

	if !tokens.Valid() {
		return nil, nil, &TokenVerificationFailedError{
			Err: err,
		}
	}

	idToken := tokens.IDToken()

	if a.config.VerboseOIDCLogging && a.logger != nil {
		a.logger.Debug("OIDC provider response", "ID token", idToken)
	}

	var allClaims map[string]interface{}
	if err := idToken.Claims(&allClaims); err != nil {
		return nil, nil, &TokenVerificationFailedError{
			Err: err,
		}
	}

	if allClaims["nonce"] != state.nonce { // TODO(sso): does this need a cast?
		return nil, nil, &TokenVerificationFailedError{
			Err: errors.New("Invalid ID token nonce."),
		}
	}
	delete(allClaims, "nonce")

	// Attempt to fetch information from the /userinfo endpoint and merge it with
	// the existing claims data. A failure to fetch additional information from this
	// endpoint will not invalidate the authorization flow.
	if tokenSource := tokens.StaticTokenSource(); tokenSource != nil {
		if err := provider.UserInfo(ctx, tokenSource, allClaims["sub"].(string), &allClaims); err != nil {
			if a.logger != nil {
				logFunc := a.logger.Warn
				if strings.Contains(err.Error(), "user info endpoint is not supported") {
					logFunc = a.logger.Info
				}
				logFunc("error reading /userinfo endpoint", "error", err)
			}
		}
	}

	if a.config.VerboseOIDCLogging && a.logger != nil {
		if c, err := json.Marshal(allClaims); err == nil {
			a.logger.Debug("OIDC provider response", "claims", string(c))
		} else {
			a.logger.Debug("OIDC provider response", "marshalling error", err.Error())
		}
	}

	c, err := a.extractClaims(allClaims)
	if err != nil {
		return nil, nil, &TokenVerificationFailedError{
			Err: err,
		}
	}

	if a.config.VerboseOIDCLogging && a.logger != nil {
		a.logger.Debug("OIDC provider response", "extracted_claims", c)
	}

	return c, state.payload, nil
}

// ProviderLoginFailedError is an error type sometimes returned from
// ClaimsFromAuthCode().
//
// It represents a failure to complete the authorization code workflow with the
// provider such as losing important OIDC parameters or a failure to fetch an
// id_token.
//
// You can check for it with errors.As().
type ProviderLoginFailedError struct {
	Err error
}

func (e *ProviderLoginFailedError) Error() string {
	return fmt.Sprintf("Provider login failed: %v", e.Err)
}

func (e *ProviderLoginFailedError) Unwrap() error { return e.Err }

// TokenVerificationFailedError is an error type sometimes returned from
// ClaimsFromAuthCode().
//
// It represents a failure to vet the returned OIDC credentials for validity
// such as the id_token not passing verification or using an mismatched nonce.
//
// You can check for it with errors.As().
type TokenVerificationFailedError struct {
	Err error
}

func (e *TokenVerificationFailedError) Error() string {
	return fmt.Sprintf("Token verification failed: %v", e.Err)
}

func (e *TokenVerificationFailedError) Unwrap() error { return e.Err }

// verifyOIDCState tests whether the provided state ID is valid and returns the
// associated state object if so. A nil state is returned if the ID is not found
// or expired. The state should only ever be retrieved once and is deleted as
// part of this request.
func (a *Authenticator) verifyOIDCState(stateID string) *oidcState {
	defer a.oidcStates.Delete(stateID)

	if stateRaw, ok := a.oidcStates.Get(stateID); ok {
		return stateRaw.(*oidcState)
	}

	return nil
}

// createOIDCState make an expiring state object, associated with a random state ID
// that is passed throughout the OAuth process. A nonce is also included in the
// auth process, and for simplicity will be identical in length/format as the state ID.
func (a *Authenticator) createOIDCState(redirectURI string, payload interface{}) (*oidc.Req, error) {
	// Get enough bytes for 2 160-bit IDs (per rfc6749#section-10.10)
	bytes, err := uuid.GenerateRandomBytes(2 * 20)
	if err != nil {
		return nil, err
	}

	stateID := fmt.Sprintf("%x", bytes[:20])
	nonce := fmt.Sprintf("%x", bytes[20:])

	// Create OIDC request object using CAP library
	// This request will be reused during token exchange
	request, error := a.oidcRequest(nonce, redirectURI, stateID)
	if error != nil {
		return nil, fmt.Errorf("Error while creating oidc req %w", error)
	}

	a.oidcStates.SetDefault(stateID, &oidcState{
		nonce:       nonce,
		redirectURI: redirectURI,
		payload:     payload,
		request:     request,
	})

	return request, nil
}

// oidcState is created when an authURL is requested. The state
// identifier is passed throughout the OAuth process.
type oidcState struct {
	nonce       string
	redirectURI string
	payload     interface{}
	request     *oidc.Req // Store the request object for later use in exchange
}

// oidcRequest builds the request to send to the HashiCorp CAP library.
// This method configures all necessary OIDC parameters including scopes,
// audiences, and security parameters like state and nonce.
func (a *Authenticator) oidcRequest(nonce, redirect string, stateID string) (*oidc.Req, error) {
	opts := []oidc.Option{
		oidc.WithNonce(nonce),
		oidc.WithState(stateID),
	}

	if len(a.config.OIDCScopes) > 0 {
		scopes := append([]string{"openid"}, a.config.OIDCScopes...)
		opts = append(opts, oidc.WithScopes(scopes...))
	}
	if len(a.config.BoundAudiences) > 0 {
		opts = append(opts, oidc.WithAudiences(a.config.BoundAudiences...))
	}
	if len(a.config.OIDCACRValues) > 0 {
		acrValues := strings.Join(a.config.OIDCACRValues, " ")
		opts = append(opts, oidc.WithACRValues(acrValues))
	}

	if a.config.OIDCClientUsePKCE != nil && *a.config.OIDCClientUsePKCE {
		verifier, err := oidc.NewCodeVerifier()
		if err != nil {
			return nil, fmt.Errorf("failed to make pkce verifier: %w", err)
		}
		opts = append(opts, oidc.WithPKCE(verifier))
	}

	if a.config.OIDCClientAssertion != nil {
		rsaKey, parseErr := jwt.ParseRSAPrivateKeyFromPEM([]byte(a.config.OIDCClientAssertion.PrivateKey.PemKey))
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse RSA private key: %w", parseErr)
		}

		// Create a JWT with the token endpoint as the audience
		var audience []string
		if len(a.config.OIDCClientAssertion.Audience) > 0 {
			audience = a.config.OIDCClientAssertion.Audience
		} else {
			audience = []string{a.config.OIDCDiscoveryURL}
		}

		var alg cass.RSAlgorithm
		switch a.config.OIDCClientAssertion.KeyAlgorithm {
		case "RS256", "": // Default to RS256 if empty
			alg = cass.RS256
		default:
			return nil, fmt.Errorf("unsupported key algorithm: %s", a.config.OIDCClientAssertion.KeyAlgorithm)
		}
		j, err := cass.NewJWTWithRSAKey(a.config.OIDCClientID, audience, alg, rsaKey)
		if err != nil {
			return nil, err
		}
		opts = append(opts, oidc.WithClientAssertionJWT(j))
	}

	req, err := oidc.NewRequest(
		oidcStateTimeout,
		redirect,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC request: %w", err)
	}

	return req, nil
}
