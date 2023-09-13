// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	APIGatewayKind = "APIGateway"
)

var (
	APIGatewayV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         APIGatewayKind,
	}

	APIGatewayType = APIGatewayV1Alpha1Type
)

func RegisterAPIGateway(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     APIGatewayV1Alpha1Type,
		Proto:    &pbmesh.APIGateway{},
		Scope:    resource.ScopeNamespace,
		ACLs:     nil, // TODO(nathancoleman)
		Mutate:   nil, // TODO(nathancoleman)
		Validate: nil, // TODO(nathancoleman)
	})
}
