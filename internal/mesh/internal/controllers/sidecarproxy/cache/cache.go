// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	"github.com/hashicorp/consul/internal/resource/mappers/selectiontracker"
	"github.com/hashicorp/consul/internal/storage"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Cache struct {
	// computedRoutes keeps track of computed routes IDs to service references it applies to.
	computedRoutes *bimapper.Mapper

	// identities keeps track of which identity a workload is mapped to.
	identities *bimapper.Mapper

	// computedDestinations keeps track of the computed explicit destinations IDs to service references that are
	// referenced in that resource.
	computedDestinations *bimapper.Mapper

	// serviceSelectorTracker keeps track of which workload selectors a service is currently using.
	serviceSelectorTracker *selectiontracker.WorkloadSelectionTracker
}

func New() *Cache {
	return &Cache{
		computedRoutes:         bimapper.New(pbmesh.ComputedRoutesType, pbcatalog.ServiceType),
		identities:             bimapper.New(pbcatalog.WorkloadType, pbauth.WorkloadIdentityType),
		computedDestinations:   bimapper.New(pbmesh.ComputedExplicitDestinationsType, pbcatalog.ServiceType),
		serviceSelectorTracker: selectiontracker.New(),
	}
}

func (c *Cache) TrackComputedDestinations(computedDestinations *types.DecodedComputedDestinations) {
	var serviceRefs []resource.ReferenceOrID

	for _, dest := range computedDestinations.Data.Destinations {
		serviceRefs = append(serviceRefs, dest.DestinationRef)
	}

	c.computedDestinations.TrackItem(computedDestinations.Resource.Id, serviceRefs)
}

func (c *Cache) UntrackComputedDestinations(computedDestinationsID *pbresource.ID) {
	c.computedDestinations.UntrackItem(computedDestinationsID)
}

func (c *Cache) UntrackComputedRoutes(computedRoutesID *pbresource.ID) {
	c.computedRoutes.UntrackItem(computedRoutesID)
}

func (c *Cache) TrackWorkload(workload *types.DecodedWorkload) {
	identityID := &pbresource.ID{
		Name:    workload.GetData().Identity,
		Tenancy: workload.GetResource().Id.Tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}
	c.identities.TrackItem(workload.GetResource().GetId(), []resource.ReferenceOrID{identityID})
}

// UntrackWorkload removes tracking for the given workload ID.
func (c *Cache) UntrackWorkload(wID *pbresource.ID) {
	c.identities.UntrackItem(wID)
}

func (c *Cache) ComputedDestinationsByService(id resource.ReferenceOrID) []*pbresource.ID {
	return c.computedDestinations.ItemIDsForLink(id)
}

func (c *Cache) trackComputedRoutes(computedRoutes *types.DecodedComputedRoutes) {
	var serviceRefs []resource.ReferenceOrID

	for _, pcr := range computedRoutes.Data.PortedConfigs {
		for _, details := range pcr.Targets {
			serviceRefs = append(serviceRefs, details.BackendRef.Ref)
		}
	}

	c.computedRoutes.TrackItem(computedRoutes.Resource.Id, serviceRefs)
}

func (c *Cache) computedRoutesByService(id resource.ReferenceOrID) []*pbresource.ID {
	return c.computedRoutes.ItemIDsForLink(id)
}

func (c *Cache) WorkloadsByWorkloadIdentity(id *pbresource.ID) []*pbresource.ID {
	return c.identities.ItemIDsForLink(id)
}

func (c *Cache) ServicesForWorkload(id *pbresource.ID) []*pbresource.ID {
	return c.serviceSelectorTracker.GetIDsForWorkload(id)
}

func (c *Cache) UntrackService(id *pbresource.ID) {
	c.serviceSelectorTracker.UntrackID(id)
}

func (c *Cache) MapComputedRoutes(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	computedRoutes, err := resource.Decode[*pbmesh.ComputedRoutes](res)
	if err != nil {
		return nil, err
	}

	ids, err := c.mapComputedRoutesToProxyStateTemplate(ctx, rt, res.Id)
	if err != nil {
		return nil, err
	}

	c.trackComputedRoutes(computedRoutes)

	return controller.MakeRequests(pbmesh.ProxyStateTemplateType, ids), nil
}

func (c *Cache) mapComputedRoutesToProxyStateTemplate(ctx context.Context, rt controller.Runtime, computedRoutesID *pbresource.ID) ([]*pbresource.ID, error) {
	// Each Destination gets a single ComputedRoutes.
	serviceID := resource.ReplaceType(pbcatalog.ServiceType, computedRoutesID)
	serviceRef := resource.Reference(serviceID, "")

	return c.mapServiceThroughDestinations(ctx, rt, serviceRef)
}

func (c *Cache) TrackService(svc *types.DecodedService) {
	c.serviceSelectorTracker.TrackIDForSelector(svc.Resource.GetId(), svc.GetData().GetWorkloads())
}

func (c *Cache) MapService(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	// Record workload selector in the cache every time we see an event for a service.
	decodedService, err := resource.Decode[*pbcatalog.Service](res)
	if err != nil {
		return nil, err
	}
	c.TrackService(decodedService)

	serviceRef := resource.Reference(res.Id, "")

	pstIDs, err := c.mapServiceThroughDestinations(ctx, rt, serviceRef)
	if err != nil {
		return nil, err
	}

	// Now walk the mesh configuration information backwards because
	// we need to find any PST that needs to DISCOVER endpoints for this
	// service as a part of mesh configuration and traffic routing.

	// Find all ComputedRoutes that reference this service.
	routeIDs := c.computedRoutesByService(serviceRef)
	for _, routeID := range routeIDs {
		// Find all Upstreams that reference a Service aligned with this ComputedRoutes.
		// Afterwards, find all Workloads selected by the Upstreams, and align a PST with those.
		ids, err := c.mapComputedRoutesToProxyStateTemplate(ctx, rt, routeID)
		if err != nil {
			return nil, err
		}

		pstIDs = append(pstIDs, ids...)
	}

	return controller.MakeRequests(pbmesh.ProxyStateTemplateType, pstIDs), nil
}

// mapServiceThroughDestinations takes an explicit
// Service and traverses back through Destinations to Workloads to
// ProxyStateTemplates.
//
// This is in a separate function so it can be chained for more complicated
// relationships.
func (c *Cache) mapServiceThroughDestinations(
	ctx context.Context,
	rt controller.Runtime,
	serviceRef *pbresource.Reference,
) ([]*pbresource.ID, error) {

	// The relationship is:
	//
	// - PST (replace type) Workload
	// - Workload (name-aligned) ComputedDestinations
	// - ComputedDestinations (contains) Service
	//
	// When we wake up for Service we should:
	//
	// - look up computed destinations for the service
	// - rewrite computed destination types to PST

	var pstIDs []*pbresource.ID

	// Get all source proxies if they're referenced in any explicit destinations from computed destinations (name-aligned with workload/PST).
	sources := c.ComputedDestinationsByService(serviceRef)
	for _, cdID := range sources {
		pstIDs = append(pstIDs, resource.ReplaceType(pbmesh.ProxyStateTemplateType, cdID))
	}

	// TODO(v2): remove this after we can do proper performant implicit upstream determination
	//
	// TODO(rb): shouldn't this instead list all Workloads that have a mesh port?
	allIDs, err := c.listAllProxyStateTemplatesTemporarily(ctx, rt, serviceRef.Tenancy)
	if err != nil {
		return nil, err
	}

	pstIDs = append(pstIDs, allIDs...)

	return pstIDs, nil
}

func (c *Cache) listAllProxyStateTemplatesTemporarily(ctx context.Context, rt controller.Runtime, tenancy *pbresource.Tenancy) ([]*pbresource.ID, error) {
	// todo (ishustava): this is a stub for now until we implement implicit destinations.
	// For tproxy, we generate requests for all proxy states in the cluster.
	// This will generate duplicate events for proxies already added above,
	// however, we expect that the controller runtime will de-dup for us.
	rsp, err := rt.Client.List(ctx, &pbresource.ListRequest{
		Type: pbmesh.ProxyStateTemplateType,
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: tenancy.Partition,
		},
	})
	if err != nil {
		return nil, err
	}

	result := make([]*pbresource.ID, 0, len(rsp.Resources))
	for _, r := range rsp.Resources {
		result = append(result, r.Id)
	}
	return result, nil
}

func (c *Cache) MapComputedTrafficPermissions(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var ctp pbauth.ComputedTrafficPermissions
	err := res.Data.UnmarshalTo(&ctp)
	if err != nil {
		return nil, err
	}

	workloadIdentityID := resource.ReplaceType(pbauth.WorkloadIdentityType, res.Id)
	ids := c.WorkloadsByWorkloadIdentity(workloadIdentityID)

	return controller.MakeRequests(pbmesh.ProxyStateTemplateType, ids), nil
}
