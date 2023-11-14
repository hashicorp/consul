// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ssoauth

import (
	"context"
	"time"

	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth"
	"github.com/hashicorp/go-hclog"
)

func init() {
	authmethod.Register("jwt", func(logger hclog.Logger, method *structs.ACLAuthMethod) (authmethod.Validator, error) {
		v, err := NewValidator(logger, method)
		if err != nil {
			return nil, err
		}
		return v, nil
	})
}

// Validator is the wrapper around the go-sso library that also conforms to the
// authmethod.Validator interface.
type Validator struct {
	name       string
	methodType string
	config     *oidcauth.Config
	logger     hclog.Logger
	oa         *oidcauth.Authenticator
}

var _ authmethod.Validator = (*Validator)(nil)

func NewValidator(logger hclog.Logger, method *structs.ACLAuthMethod) (*Validator, error) {
	if err := validateType(method.Type); err != nil {
		return nil, err
	}

	var config Config
	if err := authmethod.ParseConfig(method.Config, &config); err != nil {
		return nil, err
	}
	ssoConfig := config.convertForLibrary(method.Type)

	oa, err := oidcauth.New(ssoConfig, logger)
	if err != nil {
		return nil, err
	}

	v := &Validator{
		name:       method.Name,
		methodType: method.Type,
		config:     ssoConfig,
		logger:     logger,
		oa:         oa,
	}

	return v, nil
}

// Name implements authmethod.Validator.
func (v *Validator) Name() string { return v.name }

// Stop implements authmethod.Validator.
func (v *Validator) Stop() { v.oa.Stop() }

// ValidateLogin implements authmethod.Validator.
func (v *Validator) ValidateLogin(ctx context.Context, loginToken string) (*authmethod.Identity, error) {
	c, err := v.oa.ClaimsFromJWT(ctx, loginToken)
	if err != nil {
		return nil, err
	}

	return v.identityFromClaims(c), nil
}

func (v *Validator) identityFromClaims(c *oidcauth.Claims) *authmethod.Identity {
	id := v.NewIdentity()
	id.SelectableFields = &fieldDetails{
		Values: c.Values,
		Lists:  c.Lists,
	}
	for k, val := range c.Values {
		id.ProjectedVars["value."+k] = val
	}
	id.EnterpriseMeta = v.ssoEntMetaFromClaims(c)
	return id
}

// NewIdentity implements authmethod.Validator.
func (v *Validator) NewIdentity() *authmethod.Identity {
	// Populate selectable fields with empty values so emptystring filters
	// works. Populate projectable vars with empty values so HIL works.
	fd := &fieldDetails{
		Values: make(map[string]string),
		Lists:  make(map[string][]string),
	}
	projectedVars := make(map[string]string)
	for _, k := range v.config.ClaimMappings {
		fd.Values[k] = ""
		projectedVars["value."+k] = ""
	}
	for _, k := range v.config.ListClaimMappings {
		fd.Lists[k] = nil
	}

	return &authmethod.Identity{
		SelectableFields: fd,
		ProjectedVars:    projectedVars,
	}
}

type fieldDetails struct {
	Values map[string]string   `bexpr:"value"`
	Lists  map[string][]string `bexpr:"list"`
}

// Config is the collection of all settings that pertain to doing OIDC-based
// authentication and direct JWT-based authentication processes.
type Config struct {
	// common for type=oidc and type=jwt
	JWTSupportedAlgs    []string          `json:",omitempty"`
	BoundAudiences      []string          `json:",omitempty"`
	ClaimMappings       map[string]string `json:",omitempty"`
	ListClaimMappings   map[string]string `json:",omitempty"`
	OIDCDiscoveryURL    string            `json:",omitempty"`
	OIDCDiscoveryCACert string            `json:",omitempty"`

	// just for type=jwt
	JWKSURL              string        `json:",omitempty"`
	JWKSCACert           string        `json:",omitempty"`
	JWTValidationPubKeys []string      `json:",omitempty"`
	BoundIssuer          string        `json:",omitempty"`
	ExpirationLeeway     time.Duration `json:",omitempty"`
	NotBeforeLeeway      time.Duration `json:",omitempty"`
	ClockSkewLeeway      time.Duration `json:",omitempty"`

	enterpriseConfig `mapstructure:",squash"`
}

func (c *Config) convertForLibrary(methodType string) *oidcauth.Config {
	ssoConfig := &oidcauth.Config{
		Type: methodType,

		// common for type=oidc and type=jwt
		JWTSupportedAlgs:    c.JWTSupportedAlgs,
		BoundAudiences:      c.BoundAudiences,
		ClaimMappings:       c.ClaimMappings,
		ListClaimMappings:   c.ListClaimMappings,
		OIDCDiscoveryURL:    c.OIDCDiscoveryURL,
		OIDCDiscoveryCACert: c.OIDCDiscoveryCACert,

		// just for type=jwt
		JWKSURL:              c.JWKSURL,
		JWKSCACert:           c.JWKSCACert,
		JWTValidationPubKeys: c.JWTValidationPubKeys,
		BoundIssuer:          c.BoundIssuer,
		ExpirationLeeway:     c.ExpirationLeeway,
		NotBeforeLeeway:      c.NotBeforeLeeway,
		ClockSkewLeeway:      c.ClockSkewLeeway,
	}
	c.enterpriseConvertForLibrary(ssoConfig)
	return ssoConfig
}
