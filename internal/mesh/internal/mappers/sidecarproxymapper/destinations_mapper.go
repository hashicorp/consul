package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Mapper) MapDestinationsToProxyStateTemplate(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var destinations pbmesh.Upstreams
	err := res.Data.UnmarshalTo(&destinations)
	if err != nil {
		return nil, err
	}

	// Look up workloads for this destinations.
	sourceProxyIDs := make(map[resource.ReferenceKey]struct{})

	requests, err := mapWorkloadsBySelector(ctx, rt.Client, destinations.Workloads, res.Id.Tenancy, func(id *pbresource.ID) {
		sourceProxyIDs[resource.NewReferenceKey(id)] = struct{}{}
	})
	if err != nil {
		return nil, err
	}

	// Add this destination to destinationsCache.
	for _, destination := range destinations.Upstreams {
		destinationRef := intermediate.CombinedDestinationRef{
			ServiceRef:             destination.DestinationRef,
			Port:                   destination.DestinationPort,
			ExplicitDestinationsID: res.Id,
			SourceProxies:          sourceProxyIDs,
		}
		m.destinationsCache.WriteDestination(destinationRef)
	}

	return requests, nil
}
