// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

type DecodedNamespaceTrafficPermissions = resource.DecodedResource[*pbauth.NamespaceTrafficPermissions]

func RegisterNamespaceTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.NamespaceTrafficPermissionsType,
		Proto: &pbauth.NamespaceTrafficPermissions{},
		ACLs: &resource.ACLHooks{
			Read:  resource.DecodeAndAuthorizeRead(aclReadHookNamespaceTrafficPermissions),
			Write: resource.DecodeAndAuthorizeWrite(aclWriteHookNamespaceTrafficPermissions),
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidateNamespaceTrafficPermissions,
		Mutate:   MutateNamespaceTrafficPermissions,
		Scope:    resource.ScopeNamespace,
	})
}

func aclReadHookNamespaceTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedNamespaceTrafficPermissions) error {
	return authorizer.ToAllowAuthorizer().MeshReadAllowed(authzContext)
}

func aclWriteHookNamespaceTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedNamespaceTrafficPermissions) error {
	return authorizer.ToAllowAuthorizer().MeshWriteAllowed(authzContext)
}

var ValidateNamespaceTrafficPermissions = resource.DecodeAndValidate(validateNamespaceTrafficPermissions)

func validateNamespaceTrafficPermissions(res *DecodedNamespaceTrafficPermissions) error {
	var merr error

	if err := validateAction(res.Data); err != nil {
		merr = multierror.Append(merr, err)
	}
	if err := validatePermissions(res.Id, res.Data); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr
}

var MutateNamespaceTrafficPermissions = resource.DecodeAndMutate(mutateNamespaceTrafficPermissions)

func mutateNamespaceTrafficPermissions(res *DecodedNamespaceTrafficPermissions) (bool, error) {
	var changed bool

	for _, p := range res.Data.Permissions {
		for _, s := range p.Sources {
			if updated := normalizedTenancyForSource(s, res.Id.Tenancy); updated {
				changed = true
			}
		}
	}

	return changed, nil
}
