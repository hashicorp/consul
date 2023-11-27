package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

// RegisterMeshConfiguration takes
func RegisterMeshConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.MeshConfigurationType,
		Proto:    &pbmesh.MeshConfiguration{},
		Scope:    resource.ScopePartition,
		ACLs:     nil, // TODO NET-6423
		Mutate:   nil, // TODO NET-6425
		Validate: nil, // TODO NET-6424
	})
}
