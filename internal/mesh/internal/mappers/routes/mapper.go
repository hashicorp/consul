package routes

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	routeTypes = []*pbresource.Type{
		pbmesh.HTTPRouteType,
		pbmesh.GRPCRouteType,
		pbmesh.TCPRouteType,
	}
)

func MapDestinationPolicy(_ context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	svcID := resource.ReplaceType(pbcatalog.ServiceType, res.Id)
	return mapServiceIDToComputedRoutes(rt, svcID)
}

func MapFailoverPolicy(_ context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	svcID := resource.ReplaceType(pbcatalog.ServiceType, res.Id)
	return mapServiceIDToComputedRoutes(rt, svcID)
}

func MapService(_ context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	iter, err := rt.Cache.ListIterator(pbcatalog.FailoverPolicyType, "destinations", res.Id)
	if err != nil {
		return nil, err
	}

	var effectiveServiceIDs []*pbresource.ID
	for failover := iter.Next(); failover != nil; failover = iter.Next() {
		effectiveServiceIDs = append(effectiveServiceIDs, resource.ReplaceType(pbcatalog.ServiceType, failover.GetId()))
	}

	// (case 1) Do the direct mapping also.
	effectiveServiceIDs = append(effectiveServiceIDs, res.Id)

	var reqs []controller.Request
	for _, svcID := range effectiveServiceIDs {
		got, err := mapServiceIDToComputedRoutes(rt, svcID)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, got...)
	}

	return reqs, nil
}

func mapRequestsForBackend[T types.XRouteData](rt controller.Runtime, id *pbresource.ID) ([]controller.Request, error) {
	var zero T
	iter, err := rt.Cache.ListIterator(zero.GetResourceType(), "backend_refs", id)
	if err != nil {
		return nil, err
	}

	var out []controller.Request
	for r := iter.Next(); r != nil; r = iter.Next() {
		dec, err := resource.Decode[T](r)
		if err != nil {
			return nil, err
		}

		prefs := dec.Data.GetParentRefs()
		for _, ref := range prefs {
			if ref.Ref == nil {
				continue
			}

			out = append(out, controller.Request{ID: &pbresource.ID{
				Type:    pbmesh.ComputedRoutesType,
				Tenancy: ref.Ref.Tenancy,
				Name:    ref.Ref.Name,
			}})
		}
	}

	return out, nil
}

func mapServiceIDToComputedRoutes(rt controller.Runtime, svcID *pbresource.ID) ([]controller.Request, error) {
	var reqs []controller.Request
	reqs = append(reqs, controller.Request{
		ID: resource.ReplaceType(pbmesh.ComputedRoutesType, svcID),
	})

	httpReqs, err := mapRequestsForBackend[*pbmesh.HTTPRoute](rt, svcID)
	if err != nil {
		return nil, err
	}
	reqs = append(reqs, httpReqs...)

	grpcReqs, err := mapRequestsForBackend[*pbmesh.GRPCRoute](rt, svcID)
	if err != nil {
		return nil, err
	}
	reqs = append(reqs, grpcReqs...)

	tcpReqs, err := mapRequestsForBackend[*pbmesh.TCPRoute](rt, svcID)
	if err != nil {
		return nil, err
	}
	reqs = append(reqs, tcpReqs...)
	return reqs, nil
}
