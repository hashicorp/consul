// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MapServiceEndpointsToProxyStateTemplate maps catalog.ServiceEndpoints objects to the IDs of
// ProxyStateTemplate.
func (m *Mapper) MapServiceEndpointsToProxyStateTemplate(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	// This mapper has two jobs:
	//
	// 1. It needs to look up workload IDs from service endpoints and replace
	// them with ProxyStateTemplate type. We do this so we don't need to watch
	// Workloads to discover them, since ProxyStateTemplates are name-aligned
	// with Workloads.
	//
	// 2. It needs to find any PST that needs to DISCOVER endpoints for this
	// service as a part of mesh configuration and traffic routing.

	serviceEndpoints, err := resource.Decode[*pbcatalog.ServiceEndpoints](res)
	if err != nil {
		return nil, err
	}

	var result []controller.Request

	// (1) First, we need to generate requests from workloads this "endpoints"
	// points to so that we can re-generate proxy state for the sidecar proxy.
	for _, endpoint := range serviceEndpoints.Data.Endpoints {
		// Convert the reference to a workload to a ProxyStateTemplate ID.
		// Because these resources are name and tenancy aligned, we only need to change the type.

		// Skip service endpoints without target refs. These resources would typically be created for
		// services external to Consul, and we don't need to reconcile those as they don't have
		// associated workloads.
		if endpoint.TargetRef != nil {
			result = append(result, controller.Request{
				ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, endpoint.TargetRef),
			})
		}
	}

	// (2) Now walk the mesh configuration information backwards.

	// ServiceEndpoints -> Service
	targetServiceRef := resource.ReplaceType(pbcatalog.ServiceType, res.Id)

	// Find all ComputedRoutes that reference this service.
	routeIDs := m.computedRoutesCache.ComputedRoutesByService(targetServiceRef)
	for _, routeID := range routeIDs {
		// Find all Upstreams that reference a Service aligned with this ComputedRoutes.
		// Afterwards, find all Workloads selected by the Upstreams, and align a PST with those.
		reqs, err := m.mapComputedRoutesToProxyStateTemplate(ctx, rt, routeID)
		if err != nil {
			return nil, err
		}

		result = append(result, reqs...)
	}

	return result, nil
}
