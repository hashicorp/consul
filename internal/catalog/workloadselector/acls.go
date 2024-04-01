// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func aclReadHookResourceWithWorkloadSelector(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().ServiceReadAllowed(id.GetName(), authzContext)
}

func aclWriteHookResourceWithWorkloadSelector[T WorkloadSelecting](authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, r *resource.DecodedResource[T]) error {
	// First check service:write on the name.
	err := authorizer.ToAllowAuthorizer().ServiceWriteAllowed(r.GetId().GetName(), authzContext)
	if err != nil {
		return err
	}

	// Then also check whether we're allowed to select a service.
	for _, name := range r.Data.GetWorkloads().GetNames() {
		err = authorizer.ToAllowAuthorizer().ServiceReadAllowed(name, authzContext)
		if err != nil {
			return err
		}
	}

	for _, prefix := range r.Data.GetWorkloads().GetPrefixes() {
		err = authorizer.ToAllowAuthorizer().ServiceReadPrefixAllowed(prefix, authzContext)
		if err != nil {
			return err
		}
	}

	return nil
}

func ACLHooks[T WorkloadSelecting]() *resource.ACLHooks {
	return &resource.ACLHooks{
		Read:  aclReadHookResourceWithWorkloadSelector,
		Write: resource.DecodeAndAuthorizeWrite(aclWriteHookResourceWithWorkloadSelector[T]),
		List:  resource.NoOpACLListHook,
	}
}
