// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	GRPCRouteKind = "GRPCRoute"
)

var (
	GRPCRouteV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         GRPCRouteKind,
	}

	GRPCRouteType = GRPCRouteV1Alpha1Type
)

func RegisterGRPCRoute(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     GRPCRouteV1Alpha1Type,
		Proto:    &pbmesh.GRPCRoute{},
		Validate: nil,
		Scope:    resource.ScopeNamespace,
	})
}
