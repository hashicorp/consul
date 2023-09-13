package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ComputedGatewayKind = "ComputedGateway"
)

var (
	ComputedGatewayV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ComputedGatewayKind,
	}

	ComputedGatewayType = ComputedGatewayV1Alpha1Type
)

func RegisterComputedGateways(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ComputedGatewayV1Alpha1Type,
		Proto:    &pbmesh.ComputedGateway{},
		Scope:    resource.ScopeNamespace,
		ACLs:     nil, // TODO(nathancoleman)
		Mutate:   nil, // TODO(nathancoleman)
		Validate: nil, // TODO(nathancoleman)
	})
}
