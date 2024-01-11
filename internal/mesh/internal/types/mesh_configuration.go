// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// RegisterMeshConfiguration takes the resource registry and registers
// the MeshConfiguration resource to it.
func RegisterMeshConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.MeshConfigurationType,
		Proto:    &pbmesh.MeshConfiguration{},
		Scope:    pbresource.Scope_SCOPE_PARTITION,
		ACLs:     nil, // TODO NET-6423
		Mutate:   nil, // TODO NET-6425
		Validate: nil, // TODO NET-6424
	})
}
