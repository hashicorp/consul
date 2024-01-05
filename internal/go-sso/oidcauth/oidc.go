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

	"github.com/coreos/go-oidc"
	"github.com/hashicorp/go-uuid"
	"golang.org/x/oauth2"
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

	// "openid" is a required scope for OpenID Connect flows
	scopes := append([]string{oidc.ScopeOpenID}, a.config.OIDCScopes...)

	// Configure an OpenID Connect aware OAuth2 client
	oauth2Config := oauth2.Config{
		ClientID:     a.config.OIDCClientID,
		ClientSecret: a.config.OIDCClientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     a.provider.Endpoint(),
		Scopes:       scopes,
	}

	stateID, nonce, err := a.createOIDCState(redirectURI, statePayload)
	if err != nil {
		return "", fmt.Errorf("error generating OAuth state: %v", err)
	}

	authCodeOpts := []oauth2.AuthCodeOption{
		oidc.Nonce(nonce),
	}
	if len(a.config.OIDCACRValues) > 0 {
		authCodeOpts = append(authCodeOpts, oauth2.SetAuthURLParam("acr_values", strings.Join(a.config.OIDCACRValues, " ")))
	}

	return oauth2Config.AuthCodeURL(stateID, authCodeOpts...), nil
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

	oidcCtx := contextWithHttpClient(ctx, a.httpClient)

	var oauth2Config = oauth2.Config{
		ClientID:     a.config.OIDCClientID,
		ClientSecret: a.config.OIDCClientSecret,
		RedirectURL:  state.redirectURI,
		Endpoint:     a.provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID},
	}

	oauth2Token, err := oauth2Config.Exchange(oidcCtx, code)
	if err != nil {
		return nil, nil, &ProviderLoginFailedError{
			Err: fmt.Errorf("Error exchanging oidc code: %w", err),
		}
	}

	// Extract the ID Token from OAuth2 token.
	rawToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, nil, &TokenVerificationFailedError{
			Err: errors.New("No id_token found in response."),
		}
	}

	if a.config.VerboseOIDCLogging && a.logger != nil {
		a.logger.Debug("OIDC provider response", "ID token", rawToken)
	}

	// Parse and verify ID Token payload.
	allClaims, err := a.verifyOIDCToken(ctx, rawToken) // TODO(sso): should this use oidcCtx?
	if err != nil {
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
	if userinfo, err := a.provider.UserInfo(oidcCtx, oauth2.StaticTokenSource(oauth2Token)); err == nil {
		_ = userinfo.Claims(&allClaims)
	} else {
		if a.logger != nil {
			logFunc := a.logger.Warn
			if strings.Contains(err.Error(), "user info endpoint is not supported") {
				logFunc = a.logger.Info
			}
			logFunc("error reading /userinfo endpoint", "error", err)
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

func (a *Authenticator) verifyOIDCToken(ctx context.Context, rawToken string) (map[string]interface{}, error) {
	allClaims := make(map[string]interface{})

	oidcConfig := &oidc.Config{
		SupportedSigningAlgs: a.config.JWTSupportedAlgs,
	}
	switch a.config.authType() {
	case authOIDCFlow:
		oidcConfig.ClientID = a.config.OIDCClientID
	case authOIDCDiscovery:
		oidcConfig.SkipClientIDCheck = true
	default:
		return nil, fmt.Errorf("unsupported auth type for this verifyOIDCToken: %d", a.config.authType())
	}

	verifier := a.provider.Verifier(oidcConfig)

	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("error validating signature: %v", err)
	}

	if err := idToken.Claims(&allClaims); err != nil {
		return nil, fmt.Errorf("unable to successfully parse all claims from token: %v", err)
	}
	// TODO(sso): why isn't this strict for OIDC?
	if err := validateAudience(a.config.BoundAudiences, idToken.Audience, false); err != nil {
		return nil, fmt.Errorf("error validating claims: %v", err)
	}

	return allClaims, nil
}

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
func (a *Authenticator) createOIDCState(redirectURI string, payload interface{}) (string, string, error) {
	// Get enough bytes for 2 160-bit IDs (per rfc6749#section-10.10)
	bytes, err := uuid.GenerateRandomBytes(2 * 20)
	if err != nil {
		return "", "", err
	}

	stateID := fmt.Sprintf("%x", bytes[:20])
	nonce := fmt.Sprintf("%x", bytes[20:])

	a.oidcStates.SetDefault(stateID, &oidcState{
		nonce:       nonce,
		redirectURI: redirectURI,
		payload:     payload,
	})

	return stateID, nonce, nil
}

// oidcState is created when an authURL is requested. The state
// identifier is passed throughout the OAuth process.
type oidcState struct {
	nonce       string
	redirectURI string
	payload     interface{}
}
