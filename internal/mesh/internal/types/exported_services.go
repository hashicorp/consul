package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
)

func RegisterComputedExportedServices(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmulticluster.ComputedExportedServicesType,
		Proto: &pbmulticluster.ComputedExportedServices{},
		Scope: resource.ScopeNamespace,
	})
}
