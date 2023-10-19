// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterPartitionExportedServices(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmulticluster.PartitionExportedServicesType,
		Proto:    &pbmulticluster.PartitionExportedServices{},
		Scope:    resource.ScopePartition,
		Mutate:   MutatePartitionExportedServices,
		Validate: ValidatePartitionExportedServices,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookPartitionExportedServices,
			Write: aclWriteHookPartitionExportedServices,
			List:  aclListHookPartitionExportedServices,
		},
	})
}

func aclReadHookPartitionExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, res *pbresource.Resource) error {
	if res == nil {
		return resource.ErrNeedResource
	}
	return authorizer.ToAllowAuthorizer().MeshReadAllowed(authzContext)
}

func aclWriteHookPartitionExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().MeshWriteAllowed(authzContext)
}

func aclListHookPartitionExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext) error {
	// No-op List permission as we want to default to filtering resources
	// from the list using the Read enforcement.
	return nil
}
