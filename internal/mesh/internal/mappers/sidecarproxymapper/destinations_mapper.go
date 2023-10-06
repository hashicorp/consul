// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Mapper) MapDestinationsToProxyStateTemplate(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	destinations, err := resource.Decode[*pbmesh.Destinations](res)
	if err != nil {
		return nil, err
	}

	// Look up workloads for this destinations.
	sourceProxyIDs := make(map[resource.ReferenceKey]struct{})

	requests, err := mapSelectorToProxyStateTemplates(ctx, rt.Client, destinations.Data.Workloads, res.Id.Tenancy, func(id *pbresource.ID) {
		sourceProxyIDs[resource.NewReferenceKey(id)] = struct{}{}
	})
	if err != nil {
		return nil, err
	}

	// Add this destination to destinationsCache.
	for _, destination := range destinations.Data.Destinations {
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
