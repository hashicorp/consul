// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

type DecodedPartitionTrafficPermissions = resource.DecodedResource[*pbauth.PartitionTrafficPermissions]

func RegisterPartitionTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.PartitionTrafficPermissionsType,
		Proto: &pbauth.PartitionTrafficPermissions{},
		ACLs: &resource.ACLHooks{
			Read:  resource.DecodeAndAuthorizeRead(aclReadHookPartitionTrafficPermissions),
			Write: resource.DecodeAndAuthorizeWrite(aclWriteHookPartitionTrafficPermissions),
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidatePartitionTrafficPermissions,
		Mutate:   MutatePartitionTrafficPermissions,
		Scope:    resource.ScopePartition,
	})
}

func aclReadHookPartitionTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedPartitionTrafficPermissions) error {
	return authorizer.ToAllowAuthorizer().MeshReadAllowed(authzContext)
}

func aclWriteHookPartitionTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedPartitionTrafficPermissions) error {
	return authorizer.ToAllowAuthorizer().MeshWriteAllowed(authzContext)
}

var ValidatePartitionTrafficPermissions = resource.DecodeAndValidate(validatePartitionTrafficPermissions)

func validatePartitionTrafficPermissions(res *DecodedPartitionTrafficPermissions) error {
	var merr error

	if err := validateAction(res.Data); err != nil {
		merr = multierror.Append(merr, err)
	}
	if err := validatePermissions(res.Id, res.Data); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr
}

var MutatePartitionTrafficPermissions = resource.DecodeAndMutate(mutatePartitionTrafficPermissions)

func mutatePartitionTrafficPermissions(res *DecodedPartitionTrafficPermissions) (bool, error) {
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
