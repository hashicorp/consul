// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"fmt"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type loader struct {
	cache cache.ReadOnlyCache

	// output var
	out *RelatedResources

	// working state
	mcToLoad map[resource.ReferenceKey]struct{}
	mcDone   map[resource.ReferenceKey]struct{}
}

func LoadResourcesForComputedRoutes(
	cache cache.ReadOnlyCache,
	computedRoutesID *pbresource.ID,
) (*RelatedResources, error) {
	loader := &loader{
		cache:    cache,
		mcToLoad: make(map[resource.ReferenceKey]struct{}),
		mcDone:   make(map[resource.ReferenceKey]struct{}),
	}

	if err := loader.load(computedRoutesID); err != nil {
		return nil, err
	}

	return loader.out, nil
}

func (l *loader) requestLoad(computedRoutesID *pbresource.ID) {
	assertResourceType(pbmesh.ComputedRoutesType, computedRoutesID.Type)
	rk := resource.NewReferenceKey(computedRoutesID)

	if _, done := l.mcDone[rk]; done {
		return
	}
	l.mcToLoad[rk] = struct{}{}
}

func (l *loader) markLoaded(computedRoutesID *pbresource.ID) {
	assertResourceType(pbmesh.ComputedRoutesType, computedRoutesID.Type)
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

func (l *loader) load(computedRoutesID *pbresource.ID) error {
	l.out = NewRelatedResources()

	// Seed the graph fetch for our starting position.
	l.requestLoad(computedRoutesID)

	for {
		mcID := l.nextRequested()
		if mcID == nil {
			break
		}

		if err := l.loadOne(mcID); err != nil {
			return err
		}
	}

	return nil
}

func (l *loader) loadOne(computedRoutesID *pbresource.ID) error {
	// There is one computed routes for the entire service (perfect name alignment).
	//
	// All ports are embedded within.

	parentServiceID := resource.ReplaceType(pbcatalog.ServiceType, computedRoutesID)
	parentServiceRef := resource.Reference(parentServiceID, "")

	if err := l.loadBackendServiceInfo(parentServiceID); err != nil {
		return err
	}
	if err := l.gatherHTTPRoutesAsInput(parentServiceRef); err != nil {
		return err
	}
	if err := l.gatherGRPCRoutesAsInput(parentServiceRef); err != nil {
		return err
	}
	if err := l.gatherTCPRoutesAsInput(parentServiceRef); err != nil {
		return err
	}

	l.out.AddComputedRoutesIDs(computedRoutesID)

	l.markLoaded(computedRoutesID)

	return nil
}

func (l *loader) gatherHTTPRoutesAsInput(parentServiceRef *pbresource.Reference) error {
	httpRoutes, err := cache.ListIteratorDecoded[*pbmesh.HTTPRoute](l.cache, pbmesh.HTTPRouteType, httpRouteByParentServiceIndex.Name(), parentServiceRef)
	if err != nil {
		return err
	}
	return gatherXRoutesAsInput(l, httpRoutes, func(route *types.DecodedHTTPRoute) {
		l.out.AddHTTPRoute(route)
	})
}

func (l *loader) gatherGRPCRoutesAsInput(parentServiceRef *pbresource.Reference) error {
	grpcRoutes, err := cache.ListIteratorDecoded[*pbmesh.GRPCRoute](l.cache, pbmesh.GRPCRouteType, grpcRouteByParentServiceIndex.Name(), parentServiceRef)
	if err != nil {
		return err
	}
	return gatherXRoutesAsInput(l, grpcRoutes, func(route *types.DecodedGRPCRoute) {
		l.out.AddGRPCRoute(route)
	})
}

func (l *loader) gatherTCPRoutesAsInput(parentServiceRef *pbresource.Reference) error {
	tcpRoutes, err := cache.ListIteratorDecoded[*pbmesh.TCPRoute](l.cache, pbmesh.TCPRouteType, tcpRouteByParentServiceIndex.Name(), parentServiceRef)
	if err != nil {
		return err
	}
	return gatherXRoutesAsInput(l, tcpRoutes, func(route *types.DecodedTCPRoute) {
		l.out.AddTCPRoute(route)
	})
}

func (l *loader) loadBackendServiceInfo(svcID *pbresource.ID) error {
	service, err := cache.GetDecoded[*pbcatalog.Service](l.cache, pbcatalog.ServiceType, "id", svcID)
	if err != nil {
		return err
	}
	if service != nil {
		l.out.AddService(service)

		failoverPolicyID := resource.ReplaceType(pbcatalog.ComputedFailoverPolicyType, svcID)
		failoverPolicy, err := cache.GetDecoded[*pbcatalog.ComputedFailoverPolicy](l.cache, pbcatalog.ComputedFailoverPolicyType, "id", failoverPolicyID)
		if err != nil {
			return err
		}
		if failoverPolicy != nil {
			l.out.AddComputedFailoverPolicy(failoverPolicy)

			destRefs := failoverPolicy.Data.GetUnderlyingDestinationRefs()
			for _, destRef := range destRefs {
				destID := resource.IDFromReference(destRef)

				failService, err := cache.GetDecoded[*pbcatalog.Service](l.cache, pbcatalog.ServiceType, "id", destID)
				if err != nil {
					return err
				}
				if failService != nil {
					l.out.AddService(failService)

					if err := l.loadDestConfig(failService.Resource.Id); err != nil {
						return err
					}
				}
			}
		}

		if err := l.loadDestConfig(svcID); err != nil {
			return err
		}
	}

	return nil
}

func (l *loader) loadDestConfig(svcID *pbresource.ID) error {
	destPolicyID := resource.ReplaceType(pbmesh.DestinationPolicyType, svcID)
	destPolicy, err := cache.GetDecoded[*pbmesh.DestinationPolicy](l.cache, pbmesh.DestinationPolicyType, "id", destPolicyID)
	if err != nil {
		return err
	}
	if destPolicy != nil {
		l.out.AddDestinationPolicy(destPolicy)
	}
	return nil
}

func gatherXRoutesAsInput[T types.XRouteData](
	l *loader,
	iter cache.DecodedResourceIterator[T],
	relatedRouteCaptureFn func(*resource.DecodedResource[T]),
) error {
	for {
		route, err := iter.Next()
		if err != nil {
			return err
		} else if route == nil {
			return nil
		}

		relatedRouteCaptureFn(route)

		err = gatherSingleXRouteAsInput[T](l, route)
		if err != nil {
			return err
		}
	}
}

func gatherSingleXRouteAsInput[T types.XRouteData](l *loader, route *resource.DecodedResource[T]) error {
	for _, parentRef := range route.Data.GetParentRefs() {
		if types.IsServiceType(parentRef.Ref.Type) {
			parentComputedRoutesID := &pbresource.ID{
				Type:    pbmesh.ComputedRoutesType,
				Tenancy: parentRef.Ref.Tenancy,
				Name:    parentRef.Ref.Name,
			}
			// Note: this will only schedule things to load that have not already been loaded
			l.requestLoad(parentComputedRoutesID)
		}
	}

	for _, backendRef := range route.Data.GetUnderlyingBackendRefs() {
		if types.IsServiceType(backendRef.Ref.Type) {
			svcID := resource.IDFromReference(backendRef.Ref)
			if err := l.loadBackendServiceInfo(svcID); err != nil {
				return err
			}
		}
	}

	return nil
}

func assertResourceType(expected, actual *pbresource.Type) {
	if !resource.EqualType(expected, actual) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf(
			"expected a %q type, provided a %q type",
			resource.TypeToString(expected),
			resource.TypeToString(actual),
		))
	}
}
