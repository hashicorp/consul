// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package awsauth

import (
	"context"
	"fmt"

	iamauth "github.com/hashicorp/consul-awsauth"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
)

const (
	authMethodType string = "aws-iam"

	IAMServerIDHeaderName  string = "X-Consul-IAM-ServerID"
	GetEntityMethodHeader  string = "X-Consul-IAM-GetEntity-Method"
	GetEntityURLHeader     string = "X-Consul-IAM-GetEntity-URL"
	GetEntityHeadersHeader string = "X-Consul-IAM-GetEntity-Headers"
	GetEntityBodyHeader    string = "X-Consul-IAM-GetEntity-Body"
)

func init() {
	// register this as an available auth method type
	authmethod.Register(authMethodType, func(logger hclog.Logger, method *structs.ACLAuthMethod) (authmethod.Validator, error) {
		v, err := NewValidator(logger, method)
		if err != nil {
			return nil, err
		}
		return v, nil
	})
}

type Config struct {
	// BoundIAMPrincipalARNs are the trusted AWS IAM principal ARNs that are permitted
	// to login to the auth method. These can be the exact ARNs or wildcards. Wildcards
	// are only supported if EnableIAMEntityDetails is true.
	BoundIAMPrincipalARNs []string `json:",omitempty"`

	// EnableIAMEntityDetails will fetch the IAM User or IAM Role details to include
	// in binding rules. Required if wildcard principal ARNs are used.
	EnableIAMEntityDetails bool `json:",omitempty"`

	// IAMEntityTags are the specific IAM User or IAM Role tags to include as selectable
	// fields in the binding rule attributes. Requires EnableIAMEntityDetails = true.
	IAMEntityTags []string `json:",omitempty"`

	// ServerIDHeaderValue adds a X-Consul-IAM-ServerID header to each AWS API request.
	// This helps protect against replay attacks.
	ServerIDHeaderValue string `json:",omitempty"`

	// MaxRetries is the maximum number of retries on AWS API requests for recoverable errors.
	MaxRetries int `json:",omitempty"`
	// IAMEndpoint is the AWS IAM endpoint where iam:GetRole or iam:GetUser requests will be sent.
	// Note that the Host header in a signed request cannot be changed.
	IAMEndpoint string `json:",omitempty"`
	// STSEndpoint is the AWS STS endpoint where sts:GetCallerIdentity requests will be sent.
	// Note that the Host header in a signed request cannot be changed.
	STSEndpoint string `json:",omitempty"`

	// AllowedSTSHeaderValues is a list of additional allowed headers on the sts:GetCallerIdentity
	// request in the bearer token. A default list of necessary headers is allowed in any case.
	AllowedSTSHeaderValues []string `json:",omitempty"`
}

func (c *Config) convertForLibrary() *iamauth.Config {
	return &iamauth.Config{
		BoundIAMPrincipalARNs:  c.BoundIAMPrincipalARNs,
		EnableIAMEntityDetails: c.EnableIAMEntityDetails,
		IAMEntityTags:          c.IAMEntityTags,
		ServerIDHeaderValue:    c.ServerIDHeaderValue,
		MaxRetries:             c.MaxRetries,
		IAMEndpoint:            c.IAMEndpoint,
		STSEndpoint:            c.STSEndpoint,
		AllowedSTSHeaderValues: c.AllowedSTSHeaderValues,

		ServerIDHeaderName:     IAMServerIDHeaderName,
		GetEntityMethodHeader:  GetEntityMethodHeader,
		GetEntityURLHeader:     GetEntityURLHeader,
		GetEntityHeadersHeader: GetEntityHeadersHeader,
		GetEntityBodyHeader:    GetEntityBodyHeader,
	}
}

type Validator struct {
	name   string
	config *iamauth.Config
	logger hclog.Logger

	auth *iamauth.Authenticator
}

func NewValidator(logger hclog.Logger, method *structs.ACLAuthMethod) (*Validator, error) {
	if method.Type != authMethodType {
		return nil, fmt.Errorf("%q is not an AWS IAM auth method", method.Name)
	}

	var config Config
	if err := authmethod.ParseConfig(method.Config, &config); err != nil {
		return nil, err
	}
	iamConfig := config.convertForLibrary()

	auth, err := iamauth.NewAuthenticator(iamConfig, logger)
	if err != nil {
		return nil, err
	}

	return &Validator{
		name:   method.Name,
		config: iamConfig,
		logger: logger,
		auth:   auth,
	}, nil
}

// Name implements authmethod.Validator.
func (v *Validator) Name() string { return v.name }

// Stop implements authmethod.Validator.
func (v *Validator) Stop() {}

// ValidateLogin implements authmethod.Validator.
func (v *Validator) ValidateLogin(ctx context.Context, loginToken string) (*authmethod.Identity, error) {
	details, err := v.auth.ValidateLogin(ctx, loginToken)
	if err != nil {
		return nil, err
	}

	vars := map[string]string{
		"entity_name": details.EntityName,
		"entity_id":   details.EntityId,
		"account_id":  details.AccountId,
	}
	fields := &awsSelectableFields{
		EntityName: details.EntityName,
		EntityId:   details.EntityId,
		AccountId:  details.AccountId,
	}

	if v.config.EnableIAMEntityDetails {
		vars["entity_path"] = details.EntityPath
		fields.EntityPath = details.EntityPath
		fields.EntityTags = map[string]string{}
		for _, tag := range v.config.IAMEntityTags {
			vars["entity_tags."+tag] = details.EntityTags[tag]
			fields.EntityTags[tag] = details.EntityTags[tag]
		}
	}

	result := &authmethod.Identity{
		SelectableFields: fields,
		ProjectedVars:    vars,
		EnterpriseMeta:   nil,
	}
	return result, nil

}

func (v *Validator) NewIdentity() *authmethod.Identity {
	fields := &awsSelectableFields{
		EntityTags: map[string]string{},
	}
	vars := map[string]string{
		"entity_name": "",
		"entity_id":   "",
		"account_id":  "",
	}
	if v.config.EnableIAMEntityDetails {
		vars["entity_path"] = ""
		for _, tag := range v.config.IAMEntityTags {
			vars["entity_tags."+tag] = ""
			fields.EntityTags[tag] = ""
		}
	}
	return &authmethod.Identity{
		SelectableFields: fields,
		ProjectedVars:    vars,
	}
}

type awsSelectableFields struct {
	EntityName string `bexpr:"entity_name"`
	EntityId   string `bexpr:"entity_id"`
	AccountId  string `bexpr:"account_id"`

	EntityPath string            `bexpr:"entity_path"`
	EntityTags map[string]string `bexpr:"entity_tags"`
}
