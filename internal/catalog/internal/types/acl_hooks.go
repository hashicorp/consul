// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func aclReadHookResourceWithWorkloadSelector(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().ServiceReadAllowed(id.GetName(), authzContext)
}

func aclWriteHookResourceWithWorkloadSelector[T WorkloadSelecting](authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	if res == nil {
		return resource.ErrNeedResource
	}

	decodedService, err := resource.Decode[T](res)
	if err != nil {
		return resource.ErrNeedResource
	}

	// First check service:write on the name.
	err = authorizer.ToAllowAuthorizer().ServiceWriteAllowed(res.GetId().GetName(), authzContext)
	if err != nil {
		return err
	}

	// Then also check whether we're allowed to select a service.
	for _, name := range decodedService.GetData().GetWorkloads().GetNames() {
		err = authorizer.ToAllowAuthorizer().ServiceReadAllowed(name, authzContext)
		if err != nil {
			return err
		}
	}

	for _, prefix := range decodedService.GetData().GetWorkloads().GetPrefixes() {
		err = authorizer.ToAllowAuthorizer().ServiceReadPrefixAllowed(prefix, authzContext)
		if err != nil {
			return err
		}
	}

	return nil
}

func ACLHooksForWorkloadSelectingType[T WorkloadSelecting]() *resource.ACLHooks {
	return &resource.ACLHooks{
		Read:  aclReadHookResourceWithWorkloadSelector,
		Write: aclWriteHookResourceWithWorkloadSelector[T],
		List:  resource.NoOpACLListHook,
	}
}
