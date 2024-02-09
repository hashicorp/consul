// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedComputedImplicitDestinations = resource.DecodedResource[*pbmesh.ComputedImplicitDestinations]

func RegisterComputedImplicitDestinations(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmesh.ComputedImplicitDestinationsType,
		Proto: &pbmesh.ComputedImplicitDestinations{},
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookComputedImplicitDestinations,
			Write: aclWriteHookComputedImplicitDestinations,
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidateComputedImplicitDestinations,
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

func validateImplicitDestination(p *pbmesh.ImplicitDestination, wrapErr func(error) error) error {
	var merr error

	wrapRefErr := func(err error) error {
		return wrapErr(resource.ErrInvalidField{
			Name:    "destination_ref",
			Wrapped: err,
		})
	}

	if refErr := catalog.ValidateLocalServiceRefNoSection(p.DestinationRef, wrapRefErr); refErr != nil {
		merr = multierror.Append(merr, refErr)
	}

	if len(p.DestinationPorts) == 0 {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "destination_ports",
			Wrapped: resource.ErrEmpty,
		}))
	} else {
		for i, port := range p.DestinationPorts {
			if portErr := catalog.ValidatePortName(port); portErr != nil {
				merr = multierror.Append(merr, wrapErr(resource.ErrInvalidListElement{
					Name:    "destination_ports",
					Index:   i,
					Wrapped: portErr,
				}))
			}
		}
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
