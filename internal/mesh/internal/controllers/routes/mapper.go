// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func MapHTTPRoute(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	if !types.IsHTTPRouteType(res.Id.Type) {
		return nil, fmt.Errorf("type is not a http route type: %s", res.Id.Type)
	}
	return dependency.MapDecoded(mapXRoute[*pbmesh.HTTPRoute])(ctx, rt, res)
}

func MapGRPCRoute(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	if !types.IsGRPCRouteType(res.Id.Type) {
		return nil, fmt.Errorf("type is not a grpc route type: %s", res.Id.Type)
	}
	return dependency.MapDecoded(mapXRoute[*pbmesh.GRPCRoute])(ctx, rt, res)
}

func MapTCPRoute(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	if !types.IsTCPRouteType(res.Id.Type) {
		return nil, fmt.Errorf("type is not a tcp route type: %s", res.Id.Type)
	}
	return dependency.MapDecoded(mapXRoute[*pbmesh.TCPRoute])(ctx, rt, res)
}

func mapXRoute[T types.XRouteData](_ context.Context, _ controller.Runtime, xr *resource.DecodedResource[T]) ([]controller.Request, error) {
	refs := parentRefSliceToRefSlice(xr.Data.GetParentRefs())
	return controller.MakeRequests(pbmesh.ComputedRoutesType, refs), nil
}

func MapServiceNameAligned(
	ctx context.Context,
	rt controller.Runtime,
	res *pbresource.Resource,
) ([]controller.Request, error) {
	// Since this is name-aligned, just switch the type and find routes that
	// will route any traffic to this destination service.
	svcID := resource.ReplaceType(pbcatalog.ServiceType, res.Id)

	svc, err := rt.Cache.Get(pbcatalog.ServiceType, "id", svcID)
	if err != nil {
		return nil, err
	}

	// NOTE: this may over-trigger slightly and include extraneous
	// re-reconciles due to also traversing the failover policies backwards in
	// addition to the stuff we want.
	//
	// This is not a correctness-problem.
	return MapService(ctx, rt, svc)
}

func MapService(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	if !types.IsServiceType(res.Id.Type) {
		return nil, fmt.Errorf("type is not a service type: %s", res.Id.Type)
	}

	return dependency.MultiMapper(
		dependency.ReplaceType(pbmesh.ComputedRoutesType), // TODO: may be unnecessary
		dependency.MapperWithTransform(
			// (2) find xRoutes with the provided service as a backend; and enumerate all parent refs
			mapBackendServiceToComputedRoutes,
			// (1) find failover policies that include this as a destination; also include itself
			transformServiceToBackendServices,
		),
	)(ctx, rt, res)
}

func appendParentsFromIteratorAsComputedRoutes[T types.XRouteData](out []controller.Request, iter cache.DecodedResourceIterator[T]) ([]controller.Request, error) {
	for {
		res, err := iter.Next()
		if err != nil {
			return nil, err
		} else if res == nil {
			return out, nil
		}
		for _, ref := range res.Data.GetParentRefs() {
			out = append(out, controller.Request{
				ID: resource.ReplaceType(pbmesh.ComputedRoutesType, resource.IDFromReference(ref.Ref)),
			})
		}
	}
}

func mapBackendServiceToComputedRoutes(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var reqs []controller.Request

	// return 1 hit for the name aligned mesh config
	reqs = append(reqs, controller.Request{
		ID: resource.ReplaceType(pbmesh.ComputedRoutesType, res.Id),
	})

	// Find all routes with an explicit backend ref to this service.
	//
	// the "name aligned" inclusion above should handle the implicit default
	// destination implied by a parent ref without us having to do much more.
	httpRoutes, err := cache.ListIteratorDecoded[*pbmesh.HTTPRoute](
		rt.Cache,
		pbmesh.HTTPRouteType,
		httpRouteByBackendServiceIndex.Name(),
		res.Id,
	)
	if err != nil {
		return nil, err
	}
	reqs, err = appendParentsFromIteratorAsComputedRoutes(reqs, httpRoutes)
	if err != nil {
		return nil, err
	}

	grpcRoutes, err := cache.ListIteratorDecoded[*pbmesh.GRPCRoute](
		rt.Cache,
		pbmesh.GRPCRouteType,
		grpcRouteByBackendServiceIndex.Name(),
		res.Id,
	)
	if err != nil {
		return nil, err
	}
	reqs, err = appendParentsFromIteratorAsComputedRoutes(reqs, grpcRoutes)
	if err != nil {
		return nil, err
	}

	tcpRoutes, err := cache.ListIteratorDecoded[*pbmesh.TCPRoute](
		rt.Cache,
		pbmesh.TCPRouteType,
		tcpRouteByBackendServiceIndex.Name(),
		res.Id,
	)
	if err != nil {
		return nil, err
	}
	reqs, err = appendParentsFromIteratorAsComputedRoutes(reqs, tcpRoutes)
	if err != nil {
		return nil, err
	}

	return reqs, nil
}

func transformServiceToBackendServices(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]*pbresource.Resource, error) {
	if !types.IsServiceType(res.Id.Type) {
		return nil, fmt.Errorf("type is not a service type: %s", res.Id.Type)
	}

	// Ultimately we want to wake up a ComputedRoutes if either of the
	// following exist:
	//
	// 1. xRoute[parentRef=OUTPUT_EVENT; backendRef=INPUT_EVENT]
	// 2. xRoute[parentRef=OUTPUT_EVENT; backendRef=SOMETHING], FailoverPolicy[name=SOMETHING, destRef=INPUT_EVENT]

	// (case 2) First find all failover policies that have a reference to our input service.
	effectiveServiceIDs, err := getServicesByFailoverDestinations(rt.Cache, res.Id)
	if err != nil {
		return nil, err
	}

	// (case 1) Do the direct mapping also.
	effectiveServiceIDs = append(effectiveServiceIDs, res.Id)

	var (
		out  = make([]*pbresource.Resource, 0, len(effectiveServiceIDs))
		seen = make(map[resource.ReferenceKey]struct{})
	)
	for _, id := range effectiveServiceIDs {
		rk := resource.NewReferenceKey(id)
		if _, ok := seen[rk]; ok {
			continue
		}
		got, err := rt.Cache.Get(pbcatalog.ServiceType, "id", id)
		if err != nil {
			return nil, err
		} else if got != nil {
			out = append(out, got)
			seen[rk] = struct{}{}
		}
	}

	return out, nil
}

func getServicesByFailoverDestinations(cache cache.ReadOnlyCache, id *pbresource.ID) ([]*pbresource.ID, error) {
	iter, err := cache.ListIterator(
		pbcatalog.ComputedFailoverPolicyType,
		computedFailoverByDestRefIndex.Name(),
		id,
	)
	if err != nil {
		return nil, err
	}

	var resolved []*pbresource.ID
	for res := iter.Next(); res != nil; res = iter.Next() {
		resolved = append(resolved, resource.ReplaceType(pbcatalog.ServiceType, res.Id))
	}
	return resolved, nil
}
