// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ProxyStateTemplateKind = "ProxyStateTemplate"
)

var (
	ProxyStateTemplateV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ProxyStateTemplateKind,
	}

	ProxyStateTemplateType = ProxyStateTemplateV1Alpha1Type
)

func RegisterProxyStateTemplate(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ProxyStateTemplateV1Alpha1Type,
		Proto:    &pbmesh.ProxyStateTemplate{},
		Validate: nil,
		ACLs: &resource.ACLHooks{
			Read: func(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID) error {
				// Check service:read and operator:read permissions.
				// If service:read is not allowed, check operator:read. We want to allow both as this
				// resource is mostly useful for debuggability and we want to cover
				// the most cases that serve that purpose.
				serviceReadErr := authorizer.ToAllowAuthorizer().ServiceReadAllowed(id.Name, authzContext)
				operatorReadErr := authorizer.ToAllowAuthorizer().OperatorReadAllowed(authzContext)

				switch {
				case serviceReadErr != nil:
					return serviceReadErr
				case operatorReadErr != nil:
					return operatorReadErr
				}

				return nil
			},
			Write: func(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, p *pbresource.Resource) error {
				// Require operator:write only for "break-glass" scenarios as this resource should be mostly
				// managed by a controller.
				return authorizer.ToAllowAuthorizer().OperatorWriteAllowed(authzContext)
			},
			List: func(authorizer acl.Authorizer, tenancy *pbresource.Tenancy) error {
				// No-op List permission as we want to default to filtering resources
				// from the list using the Read enforcement.
				return nil
			},
		},
	})
}
