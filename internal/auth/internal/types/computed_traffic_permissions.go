// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedComputedTrafficPermissions = resource.DecodedResource[*pbauth.ComputedTrafficPermissions]

func RegisterComputedTrafficPermission(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.ComputedTrafficPermissionsType,
		Proto: &pbauth.ComputedTrafficPermissions{},
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookComputedTrafficPermissions,
			Write: aclWriteHookComputedTrafficPermissions,
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidateComputedTrafficPermissions,
		Scope:    resource.ScopeNamespace,
	})
}

var ValidateComputedTrafficPermissions = resource.DecodeAndValidate(validateComputedTrafficPermissions)

func validateComputedTrafficPermissions(res *DecodedComputedTrafficPermissions) error {
	var merr error

	for i, permission := range res.Data.AllowPermissions {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "allow_permissions",
				Index:   i,
				Wrapped: err,
			}
		}
		if err := validatePermission(permission, res.Id, wrapErr); err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	for i, permission := range res.Data.DenyPermissions {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "deny_permissions",
				Index:   i,
				Wrapped: err,
			}
		}
		if err := validatePermission(permission, res.Id, wrapErr); err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	return merr
}

func aclReadHookComputedTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().TrafficPermissionsReadAllowed(id.Name, authzContext)
}

func aclWriteHookComputedTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().TrafficPermissionsWriteAllowed(res.Id.Name, authzContext)
}
