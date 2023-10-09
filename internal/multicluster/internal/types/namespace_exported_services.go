// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterNamespaceExportedServices(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmulticluster.NamespaceExportedServicesType,
		Proto: &pbmulticluster.NamespaceExportedServices{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookNamespaceExportedServices,
			Write: aclWriteHookNamespaceExportedServices,
			List:  aclListHookNamespaceExportedServices,
		},
	})
}

func aclReadHookNamespaceExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().MeshReadAllowed(authzContext)
}

func aclWriteHookNamespaceExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().MeshWriteAllowed(authzContext)
}

func aclListHookNamespaceExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext) error {
	return nil
}
