// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package implicitdestinations

import (
	"context"

	"golang.org/x/exp/maps"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/meshindexes"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type mapAndTransformer struct {
	globalDefaultAllow bool
}

// Note: these MapZZZ functions ignore the bound refs.

func (m *mapAndTransformer) MapComputedTrafficPermissions(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	// Summary: CTP <list> WI[source] <align> CID

	dm := dependency.MapDecoded[*pbauth.ComputedTrafficPermissions](
		// (1) turn CTP -> WI[source]
		m.mapComputedTrafficPermissionsToSourceWorkloadIdentities,
	)
	return dependency.WrapAndReplaceType(pbmesh.ComputedImplicitDestinationsType, dm)(ctx, rt, res)
}

func (m *mapAndTransformer) MapService(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	// Summary: SVC[backend] <list> WI[backend] <align> CTP <list> WI[source] <align> CID

	dm := dependency.MapperWithTransform(
		// (2) turn WI[backend] -> CTP -> WI[source]
		m.mapBackendWorkloadIdentityToSourceWorkloadIdentity,
		// (1) turn SVC[backend] => WI[backend]
		m.transformServiceToWorkloadIdentities,
	)
	return dependency.WrapAndReplaceType(pbmesh.ComputedImplicitDestinationsType, dm)(ctx, rt, res)
}

func (m *mapAndTransformer) MapComputedRoutes(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	// Summary: CR <list> SVC[backend] <list> WI[backend] <align> CTP <list> WI[source] <align> CID

	dm := dependency.MapperWithTransform(
		// (3) turn WI[backend] -> CTP -> WI[source]
		m.mapBackendWorkloadIdentityToSourceWorkloadIdentity,
		dependency.TransformChain(
			// (1) Turn CR -> SVC[backend]
			m.transformComputedRoutesToBackendServiceRefs,
			// (2) Turn SVC[backend] -> WI[backend]
			m.transformServiceToWorkloadIdentities,
		),
	)
	return dependency.WrapAndReplaceType(pbmesh.ComputedImplicitDestinationsType, dm)(ctx, rt, res)
}

func (m *mapAndTransformer) mapComputedTrafficPermissionsToSourceWorkloadIdentities(ctx context.Context, rt controller.Runtime, ctp *types.DecodedComputedTrafficPermissions) ([]controller.Request, error) {
	refs, err := m.getSourceWorkloadIdentitiesFromCTPWithWildcardExpansion(rt.Cache, ctp)
	if err != nil {
		return nil, err
	}
	return controller.MakeRequests(pbauth.WorkloadIdentityType, refs), nil
}

func (m *mapAndTransformer) getSourceWorkloadIdentitiesFromCTPWithWildcardExpansion(
	cache cache.ReadOnlyCache,
	ctp *types.DecodedComputedTrafficPermissions,
) ([]*pbresource.Reference, error) {
	if ctp.Data.IsDefault && m.globalDefaultAllow {
		return listAllWorkloadIdentities(cache, &pbresource.Tenancy{
			Partition: storage.Wildcard,
			Namespace: storage.Wildcard,
		})
	}

	exact, wildNames, wildNS := getSourceWorkloadIdentitiesFromCTP(ctp)

	for _, wildTenancy := range wildNames {
		got, err := listAllWorkloadIdentities(cache, wildTenancy)
		if err != nil {
			return nil, err
		}
		for _, ref := range got {
			rk := resource.NewReferenceKey(ref)
			if _, ok := exact[rk]; !ok {
				exact[rk] = ref
			}
		}
	}

	for _, wildPartition := range wildNS {
		got, err := listAllWorkloadIdentities(cache, &pbresource.Tenancy{
			Partition: wildPartition,
			Namespace: storage.Wildcard,
		})
		if err != nil {
			return nil, err
		}
		for _, ref := range got {
			rk := resource.NewReferenceKey(ref)
			if _, ok := exact[rk]; !ok {
				exact[rk] = ref
			}
		}
	}

	return maps.Values(exact), nil
}

func (m *mapAndTransformer) mapBackendWorkloadIdentityToSourceWorkloadIdentity(ctx context.Context, rt controller.Runtime, wiRes *pbresource.Resource) ([]controller.Request, error) {
	ctpID := resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wiRes.Id)

	ctp, err := cache.GetDecoded[*pbauth.ComputedTrafficPermissions](rt.Cache, pbauth.ComputedTrafficPermissionsType, "id", ctpID)
	if err != nil {
		return nil, err
	} else if ctp == nil {
		return nil, nil
	}

	return m.mapComputedTrafficPermissionsToSourceWorkloadIdentities(ctx, rt, ctp)
}

func (m *mapAndTransformer) transformServiceToWorkloadIdentities(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]*pbresource.Resource, error) {
	// This is deliberately thin b/c WI's have no body, and we'll pass this to
	// another transformer immediately anyway, so it's largely an opaque
	// carrier for the WI name string only.

	wiIDs := meshindexes.GetWorkloadIdentitiesFromService(res)

	out := make([]*pbresource.Resource, 0, len(wiIDs))
	for _, wiID := range wiIDs {
		wiLite := &pbresource.Resource{
			Id: resource.IDFromReference(wiID),
		}
		out = append(out, wiLite)
	}

	return out, nil
}

func (m *mapAndTransformer) transformComputedRoutesToBackendServiceRefs(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]*pbresource.Resource, error) {
	cr, err := resource.Decode[*pbmesh.ComputedRoutes](res)
	if err != nil {
		return nil, err
	}

	svcRefs := meshindexes.GetBackendServiceRefsFromComputedRoutes(cr)

	out := make([]*pbresource.Resource, 0, len(svcRefs))
	for _, svcRef := range svcRefs {
		svc, err := rt.Cache.Get(pbcatalog.ServiceType, "id", svcRef)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			out = append(out, svc)
		}
	}
	return out, nil
}
