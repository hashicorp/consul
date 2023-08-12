// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	HTTPRouteKind = "HTTPRoute"
)

var (
	HTTPRouteV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         HTTPRouteKind,
	}

	HTTPRouteType = HTTPRouteV1Alpha1Type
)

func RegisterHTTPRoute(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     HTTPRouteV1Alpha1Type,
		Proto:    &pbmesh.HTTPRoute{},
		Validate: nil,
	})
}
