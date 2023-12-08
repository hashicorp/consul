// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
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
	// TODO: Once namespaces are supported in CE, we can simplify this
	allowAuthz := authorizer.ToAllowAuthorizer()
	if nsAuthz, ok := any(allowAuthz).(interface {
		NamespaceReadAllowed(string, *acl.AuthorizerContext) error
	}); ok {
		return nsAuthz.NamespaceReadAllowed(res.Id.Tenancy.Namespace, authzContext)
	}
	// Fall back to operator:read in CE
	return allowAuthz.OperatorReadAllowed(authzContext)
}

func aclWriteHookNamespaceTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedNamespaceTrafficPermissions) error {
	// TODO: Once namespaces are supported in CE, we can simplify this
	allowAuthz := authorizer.ToAllowAuthorizer()
	if nsAuthz, ok := any(allowAuthz).(interface {
		NamespaceWriteAllowed(string, *acl.AuthorizerContext) error
	}); ok {
		return nsAuthz.NamespaceWriteAllowed(res.Id.Tenancy.Namespace, authzContext)
	}
	// Fall back to operator:write in CE
	return allowAuthz.OperatorWriteAllowed(authzContext)
}

var ValidateNamespaceTrafficPermissions = resource.DecodeAndValidate(validateNamespaceTrafficPermissions)

func validateNamespaceTrafficPermissions(res *DecodedNamespaceTrafficPermissions) error {
	var merr error

	merr = validateAction(merr, res.Data)
	merr = validatePermissions(merr, res.Data)

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
