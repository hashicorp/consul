// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ComputedExportedServicesName = "global"
)

func RegisterComputedExportedServices(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmulticluster.ComputedExportedServicesType,
		Proto:    &pbmulticluster.ComputedExportedServices{},
		Scope:    resource.ScopePartition,
		Validate: ValidateComputedExportedServices,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookComputedExportedServices,
			Write: aclWriteHookComputedExportedServices,
			List:  aclListHookComputedExportedServices,
		},
	})
}

func aclReadHookComputedExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, _ *pbresource.ID, res *pbresource.Resource) error {
	if res == nil {
		return resource.ErrNeedResource
	}
	return authorizer.ToAllowAuthorizer().MeshReadAllowed(authzContext)
}

func aclWriteHookComputedExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, _ *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().MeshWriteAllowed(authzContext)
}

func aclListHookComputedExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext) error {
	// No-op List permission as we want to default to filtering resources
	// from the list using the Read enforcement.
	return nil
}
