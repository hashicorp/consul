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

type DecodedComputedImplicitDestinations = resource.DecodedResource[*pbauth.ComputedImplicitDestinations]

func RegisterImplicitDestinations(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.ComputedImplicitDestinationsType,
		Proto: &pbauth.ComputedTrafficPermissions{},
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookComputedImplicitDestinations,
			Write: aclWriteHookComputedImplicitDestinations,
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidateComputedTrafficPermissions,
		Scope:    resource.ScopeNamespace,
	})
}

var ValidateComputedImplicitDestinations = resource.DecodeAndValidate(validateComputedImplicitDestinations)

func validateComputedImplicitDestinations(res *DecodedComputedImplicitDestinations) error {
	var merr error
	for i, implDest := range res.Data.Destinations {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "destinations",
				Index:   i,
				Wrapped: err,
			}
		}
		if err := validateImplicitDestination(implDest, wrapErr); err != nil {
			merr = multierror.Append(merr, err)
		}
	}
	return merr
}

func validateImplicitDestination(p *pbauth.ImplicitDestination, wrapErr func(error) error) error {
	// every imp_dest needs a service ref and at least 1 workload identity, because traffic permissions
	// only resolve to workload identity
	var merr error
	if p.ServiceRef == nil {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "service_ref",
			Wrapped: resource.ErrEmpty,
		})
	}
	if len(p.IdentityRefs) < 1 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "identity_refs",
			Wrapped: resource.ErrEmpty,
		})
	}
	return merr
}

func aclReadHookComputedImplicitDestinations(
	authorizer acl.Authorizer,
	authzCtx *acl.AuthorizerContext,
	id *pbresource.ID,
	res *pbresource.Resource,
) error {
	if id != nil {
		return authorizer.ToAllowAuthorizer().IdentityReadAllowed(id.Name, authzCtx)
	}
	if res != nil {
		return authorizer.ToAllowAuthorizer().IdentityReadAllowed(res.Id.Name, authzCtx)
	}
	return resource.ErrNeedResource
}

func aclWriteHookComputedImplicitDestinations(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().OperatorWriteAllowed(authzContext)
}
