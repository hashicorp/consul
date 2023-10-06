// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterWorkloadIdentity(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.WorkloadIdentityType,
		Proto: &pbauth.WorkloadIdentity{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookWorkloadIdentity,
			Write: aclWriteHookWorkloadIdentity,
			List:  aclListHookWorkloadIdentity,
		},
		Validate: nil,
	})
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
	return resource.ErrNeedData
}

func aclWriteHookWorkloadIdentity(
	authorizer acl.Authorizer,
	authzCtx *acl.AuthorizerContext,
	res *pbresource.Resource) error {
	if res == nil {
		return resource.ErrNeedData
	}
	return authorizer.ToAllowAuthorizer().IdentityWriteAllowed(res.Id.Name, authzCtx)
}

func aclListHookWorkloadIdentity(authorizer acl.Authorizer, context *acl.AuthorizerContext) error {
	// No-op List permission as we want to default to filtering resources
	// from the list using the Read enforcement
	return nil
}
