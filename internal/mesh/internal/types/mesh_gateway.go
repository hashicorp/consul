// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func RegisterMeshGateway(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.MeshGatewayType,
		Proto:    &pbmesh.MeshGateway{},
		Scope:    resource.ScopePartition,
		ACLs:     nil, // TODO NET-6416
		Mutate:   nil, // TODO NET-6418
		Validate: nil, // TODO NET-6417
	})
}
