// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	MeshGatewayKind = "MeshGateway"
)

var (
	MeshGatewayV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         MeshGatewayKind,
	}

	MeshGatewayType = MeshGatewayV1Alpha1Type
)

func RegisterMeshGateway(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     MeshGatewayV1Alpha1Type,
		Proto:    &pbmesh.MeshGateway{},
		Scope:    resource.ScopeNamespace,
		Mutate:   nil,
		Validate: nil,
	})
}
