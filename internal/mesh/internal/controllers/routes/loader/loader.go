// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/xroutemapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type loader struct {
	mapper *xroutemapper.Mapper

	mem *memoizingLoader

	// output var
	out *RelatedResources

	// working state
	mcToLoad map[resource.ReferenceKey]struct{}
	mcDone   map[resource.ReferenceKey]struct{}
}

func LoadResourcesForComputedRoutes(
	ctx context.Context,
	loggerFor func(*pbresource.ID) hclog.Logger,
	client pbresource.ResourceServiceClient,
	mapper *xroutemapper.Mapper,
	computedRoutesID *pbresource.ID,
) (*RelatedResources, error) {
	if loggerFor == nil {
		loggerFor = func(_ *pbresource.ID) hclog.Logger {
			return hclog.NewNullLogger()
		}
	}
	loader := &loader{
		mapper:   mapper,
		mem:      newMemoizingLoader(client),
		mcToLoad: make(map[resource.ReferenceKey]struct{}),
		mcDone:   make(map[resource.ReferenceKey]struct{}),
	}

	if err := loader.load(ctx, loggerFor, client, computedRoutesID); err != nil {
		return nil, err
	}

	return loader.out, nil
}

func (l *loader) requestLoad(computedRoutesID *pbresource.ID) {
	if !resource.EqualType(computedRoutesID.Type, types.ComputedRoutesType) {
		panic("input must be a ComputedRoutes type")
	}
	rk := resource.NewReferenceKey(computedRoutesID)

	if _, done := l.mcDone[rk]; done {
		return
	}
	l.mcToLoad[rk] = struct{}{}
}

func (l *loader) markLoaded(computedRoutesID *pbresource.ID) {
	if !resource.EqualType(computedRoutesID.Type, types.ComputedRoutesType) {
		panic("input must be a ComputedRoutes type")
	}
	rk := resource.NewReferenceKey(computedRoutesID)

	l.mcDone[rk] = struct{}{}
	delete(l.mcToLoad, rk)
}

func (l *loader) nextRequested() *pbresource.ID {
	for rk := range l.mcToLoad {
		return rk.ToID()
	}
	return nil
}

func (l *loader) load(
	ctx context.Context,
	loggerFor func(*pbresource.ID) hclog.Logger,
	client pbresource.ResourceServiceClient,
	computedRoutesID *pbresource.ID,
) error {
	l.out = NewRelatedResources()

	// Seed the graph fetch for our starting position.
	l.requestLoad(computedRoutesID)

	for {
		mcID := l.nextRequested()
		if mcID == nil {
			break
		}

		if err := l.loadOne(ctx, loggerFor, client, mcID); err != nil {
			return err
		}
	}

	return nil
}

func (l *loader) loadOne(
	ctx context.Context,
	loggerFor func(*pbresource.ID) hclog.Logger,
	client pbresource.ResourceServiceClient,
	computedRoutesID *pbresource.ID,
) error {
	logger := loggerFor(computedRoutesID)

	// There is one computed routes for the entire service (perfect name alignment).
	//
	// All ports are embedded within.

	parentServiceID := changeResourceType(computedRoutesID, catalog.ServiceType)
	parentServiceRef := resource.Reference(parentServiceID, "")

	if err := l.loadUpstreamService(ctx, logger, parentServiceID); err != nil {
		return err
	}

	if err := l.gatherXRoutesAsInput(ctx, logger, computedRoutesID, parentServiceRef); err != nil {
		return err
	}

	l.out.AddComputedRoutesIDs(computedRoutesID)

	l.markLoaded(computedRoutesID)

	return nil
}

func (l *loader) gatherXRoutesAsInput(
	ctx context.Context,
	logger hclog.Logger,
	computedRoutesID *pbresource.ID,
	parentServiceRef *pbresource.Reference,
) error {
	routeIDs := l.mapper.RouteIDsByParentServiceRef(parentServiceRef)

	// read the xRoutes
	for _, routeID := range routeIDs {
		switch {
		case resource.EqualType(routeID.Type, types.HTTPRouteType):
			route, err := l.mem.GetHTTPRoute(ctx, routeID)
			if err != nil {
				return fmt.Errorf("the resource service has returned an unexpected error loading %s: %w", routeID, err)
			}
			var routeData types.XRouteData
			if route != nil {
				routeData = route.Data
			}
			err = l.gatherSingleXRouteAsInput(ctx, logger, computedRoutesID, routeID, routeData, func() {
				l.out.AddResource(route)
			})
			if err != nil {
				return fmt.Errorf("the resource service has returned an unexpected error loading %s: %w", routeID, err)
			}
		case resource.EqualType(routeID.Type, types.GRPCRouteType):
			route, err := l.mem.GetGRPCRoute(ctx, routeID)
			if err != nil {
				return fmt.Errorf("the resource service has returned an unexpected error loading %s: %w", routeID, err)
			}
			var routeData types.XRouteData
			if route != nil {
				routeData = route.Data
			}
			err = l.gatherSingleXRouteAsInput(ctx, logger, computedRoutesID, routeID, routeData, func() {
				l.out.AddResource(route)
			})
			if err != nil {
				return fmt.Errorf("the resource service has returned an unexpected error loading %s: %w", routeID, err)
			}
		case resource.EqualType(routeID.Type, types.TCPRouteType):
			route, err := l.mem.GetTCPRoute(ctx, routeID)
			if err != nil {
				return fmt.Errorf("the resource service has returned an unexpected error loading %s: %w", routeID, err)
			}
			var routeData types.XRouteData
			if route != nil {
				routeData = route.Data
			}
			err = l.gatherSingleXRouteAsInput(ctx, logger, computedRoutesID, routeID, routeData, func() {
				l.out.AddResource(route)
			})
			if err != nil {
				return fmt.Errorf("the resource service has returned an unexpected error loading %s: %w", routeID, err)
			}
		default:
			logger.Warn("skipping xRoute reference of unknown type", "ID", resource.IDToString(routeID))
			continue
		}
	}

	return nil
}

func (l *loader) loadUpstreamService(
	ctx context.Context,
	logger hclog.Logger,
	svcID *pbresource.ID,
) error {
	logger = logger.With("service-id", resource.IDToString(svcID))

	service, err := l.mem.GetService(ctx, svcID)
	if err != nil {
		logger.Error("error retrieving the service", "serviceID", svcID, "error", err)
		return err
	}
	if service != nil {
		l.out.AddResource(service)

		failoverPolicyID := changeResourceType(svcID, catalog.FailoverPolicyType)
		failoverPolicy, err := l.mem.GetFailoverPolicy(ctx, failoverPolicyID)
		if err != nil {
			logger.Error("error retrieving the failover policy", "failoverPolicyID", failoverPolicyID, "error", err)
			return err
		}
		if failoverPolicy != nil {
			l.mapper.TrackFailoverPolicy(failoverPolicy)
			l.out.AddResource(failoverPolicy)

			destRefs := failoverPolicy.Data.GetUnderlyingDestinationRefs()
			for _, destRef := range destRefs {
				destID := resource.IDFromReference(destRef)

				failService, err := l.mem.GetService(ctx, destID)
				if err != nil {
					logger.Error("error retrieving a failover destination service",
						"serviceID", destID, "error", err)
					return err
				}
				if failService != nil {
					l.out.AddResource(failService)
				}
			}
		} else {
			l.mapper.UntrackFailoverPolicy(failoverPolicyID)
		}

		destPolicyID := changeResourceType(svcID, types.DestinationPolicyType)
		destPolicy, err := l.mem.GetDestinationPolicy(ctx, destPolicyID)
		if err != nil {
			logger.Error("error retrieving the destination config", "destPolicyID", destPolicyID, "error", err)
			return err
		}
		if destPolicy != nil {
			l.out.AddResource(destPolicy)
		}
	}

	return nil
}

func (l *loader) gatherSingleXRouteAsInput(
	ctx context.Context,
	logger hclog.Logger,
	computedRoutesID *pbresource.ID,
	routeID *pbresource.ID,
	route types.XRouteData,
	relatedRouteCaptureFn func(),
) error {
	if route == nil {
		logger.Trace("XRoute has been deleted")
		l.mapper.UntrackXRoute(routeID)
		return nil
	}
	l.mapper.TrackXRoute(routeID, route)

	relatedRouteCaptureFn()

	for _, parentRef := range route.GetParentRefs() {
		if types.IsServiceType(parentRef.Ref.Type) {
			parentComputedRoutesID := &pbresource.ID{
				Type:    types.ComputedRoutesType,
				Tenancy: parentRef.Ref.Tenancy,
				Name:    parentRef.Ref.Name,
			}
			// Note: this will only schedule things to load that have not already been loaded
			l.requestLoad(parentComputedRoutesID)
		}
	}

	for _, backendRef := range route.GetUnderlyingBackendRefs() {
		if types.IsServiceType(backendRef.Ref.Type) {
			svcID := resource.IDFromReference(backendRef.Ref)
			if err := l.loadUpstreamService(ctx, logger, svcID); err != nil {
				return err
			}
		}
	}

	return nil
}

func changeResourceType(id *pbresource.ID, newType *pbresource.Type) *pbresource.ID {
	return &pbresource.ID{
		Type:    newType,
		Tenancy: id.Tenancy,
		Name:    id.Name,
	}
}
