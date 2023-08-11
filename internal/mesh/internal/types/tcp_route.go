// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	TCPRouteKind = "TCPRoute"
)

var (
	TCPRouteV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         TCPRouteKind,
	}

	TCPRouteType = TCPRouteV1Alpha1Type
)

func RegisterTCPRoute(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     TCPRouteV1Alpha1Type,
		Proto:    &pbmesh.TCPRoute{},
		Validate: nil,
	})
}
