// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterMeshGateway(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmesh.MeshGatewayType,
		Proto: &pbmesh.MeshGateway{},
		Scope: resource.ScopePartition,
		ACLs: &resource.ACLHooks{
			Read: func(authorizer acl.Authorizer, context *acl.AuthorizerContext, _ *pbresource.ID, _ *pbresource.Resource) error {
				return authorizer.ToAllowAuthorizer().MeshReadAllowed(context)
			},
			Write: func(authorizer acl.Authorizer, context *acl.AuthorizerContext, _ *pbresource.Resource) error {
				return authorizer.ToAllowAuthorizer().MeshWriteAllowed(context)
			},
			List: resource.NoOpACLListHook,
		},
		Mutate:   nil, // TODO NET-6425
		Validate: nil, // TODO NET-6424
	})
}
