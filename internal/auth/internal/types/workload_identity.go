// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedWorkloadIdentity = resource.DecodedResource[*pbauth.WorkloadIdentity]

func RegisterWorkloadIdentity(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.WorkloadIdentityType,
		Proto: &pbauth.WorkloadIdentity{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookWorkloadIdentity,
			Write: aclWriteHookWorkloadIdentity,
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidateWorkloadIdentity,
	})
}

var ValidateWorkloadIdentity = resource.DecodeAndValidate(validateWorkloadIdentity)

func validateWorkloadIdentity(res *DecodedWorkloadIdentity) error {
	// currently the WorkloadIdentity type has no fields.
	return nil
}

func aclReadHookWorkloadIdentity(
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

func aclWriteHookWorkloadIdentity(
	authorizer acl.Authorizer,
	authzCtx *acl.AuthorizerContext,
	res *pbresource.Resource) error {
	if res == nil {
		return resource.ErrNeedResource
	}
	return authorizer.ToAllowAuthorizer().IdentityWriteAllowed(res.Id.Name, authzCtx)
}
