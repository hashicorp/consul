// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterNamespaceExportedServices(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmulticluster.NamespaceExportedServicesType,
		Proto:    &pbmulticluster.NamespaceExportedServices{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateNamespaceExportedServices,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookNamespaceExportedServices,
			Write: aclWriteHookNamespaceExportedServices,
			List:  resource.NoOpACLListHook,
		},
	})
}

func aclReadHookNamespaceExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, res *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().MeshReadAllowed(authzContext)
}

func aclWriteHookNamespaceExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().MeshWriteAllowed(authzContext)
}
