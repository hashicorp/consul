// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"

	"github.com/hashicorp/go-multierror"
	hcpresource "github.com/hashicorp/hcp-sdk-go/resource"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedLink = resource.DecodedResource[*pbhcp.Link]

const (
	LinkName             = "global"
	MetadataSourceKey    = "source"
	MetadataSourceConfig = "config"
)

var (
	errLinkConfigurationName = errors.New("only a single Link resource is allowed and it must be named global")
	errInvalidHCPResourceID  = errors.New("could not parse, invalid format")
)

func RegisterLink(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbhcp.LinkType,
		Proto:    &pbhcp.Link{},
		Scope:    resource.ScopeCluster,
		Validate: ValidateLink,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookLink,
			Write: aclWriteHookLink,
			List:  aclListHookLink,
		},
	})
}

func aclReadHookLink(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, _ *pbresource.ID, _ *pbresource.Resource) error {
	err := authorizer.ToAllowAuthorizer().OperatorReadAllowed(authzContext)
	if err != nil {
		return err
	}
	return nil
}

func aclWriteHookLink(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, _ *pbresource.Resource) error {
	err := authorizer.ToAllowAuthorizer().OperatorWriteAllowed(authzContext)
	if err != nil {
		return err
	}

	err = authorizer.ToAllowAuthorizer().ACLWriteAllowed(authzContext)
	if err != nil {
		return err
	}

	return nil
}

func aclListHookLink(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext) error {
	err := authorizer.ToAllowAuthorizer().OperatorReadAllowed(authzContext)
	if err != nil {
		return err
	}
	return nil
}

var ValidateLink = resource.DecodeAndValidate(validateLink)

func validateLink(res *DecodedLink) error {
	var err error

	if res.Id.Name != LinkName {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "name",
			Wrapped: errLinkConfigurationName,
		})
	}

	if res.Data.ClientId == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "client_id",
			Wrapped: resource.ErrMissing,
		})
	}

	if res.Data.ClientSecret == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "client_secret",
			Wrapped: resource.ErrMissing,
		})
	}

	if res.Data.ResourceId == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "resource_id",
			Wrapped: resource.ErrMissing,
		})
	} else {
		_, parseErr := hcpresource.FromString(res.Data.ResourceId)
		if parseErr != nil {
			err = multierror.Append(err, resource.ErrInvalidField{
				Name:    "resource_id",
				Wrapped: errInvalidHCPResourceID,
			})
		}
	}

	return err
}
