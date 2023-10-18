package servicenamealigned

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

func Map(_ context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	svcID := resource.ReplaceType(pbcatalog.ServiceType, res.Id)

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
