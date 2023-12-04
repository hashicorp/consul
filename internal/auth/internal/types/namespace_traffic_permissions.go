// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

type DecodedNamespaceTrafficPermissions = resource.DecodedResource[*pbauth.NamespaceTrafficPermissions]

func RegisterNamespaceTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.NamespaceTrafficPermissionsType,
		Proto: &pbauth.NamespaceTrafficPermissions{},
		ACLs: &resource.ACLHooks{
			Read:  resource.DecodeAndAuthorizeRead(aclReadHookTrafficPermissions),
			Write: resource.DecodeAndAuthorizeWrite(aclWriteHookTrafficPermissions), // TODO(chrisk): Should this require higher privilege?
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidateNamespaceTrafficPermissions,
		Mutate:   MutateNamespaceTrafficPermissions,
		Scope:    resource.ScopeNamespace,
	})
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
