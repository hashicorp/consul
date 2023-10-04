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

func RegisterComputedTrafficPermission(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.ComputedTrafficPermissionsType,
		Proto: &pbauth.ComputedTrafficPermissions{},
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookComputedTrafficPermissions,
			Write: aclWriteHookComputedTrafficPermissions,
			List:  aclListHookComputedTrafficPermissions,
		},
		Validate: ValidateComputedTrafficPermissions,
		Scope:    resource.ScopeNamespace,
	})
}

func ValidateComputedTrafficPermissions(res *pbresource.Resource) error {
	var ctp pbauth.ComputedTrafficPermissions

	if err := res.Data.UnmarshalTo(&ctp); err != nil {
		return resource.NewErrDataParse(&ctp, err)
	}

	var merr error

	for i, permission := range ctp.AllowPermissions {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "allow_permissions",
				Index:   i,
				Wrapped: err,
			}
		}
		if err := validatePermission(permission, wrapErr); err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	for i, permission := range ctp.DenyPermissions {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "deny_permissions",
				Index:   i,
				Wrapped: err,
			}
		}
		if err := validatePermission(permission, wrapErr); err != nil {
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

func aclListHookComputedTrafficPermissions(_ acl.Authorizer, _ *acl.AuthorizerContext) error {
	// No-op List permission as we want to default to filtering resources
	// from the list using the Read enforcement
	return nil
}
