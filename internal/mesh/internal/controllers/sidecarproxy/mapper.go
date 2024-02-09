// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxy

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/mesh/internal/meshindexes"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	selectedWorkloadsIndexName = "selected-workloads"
)

// Cache: reverse CED => Service(destination)
var computedExplicitDestinationsByServiceIndex = indexers.RefOrIDIndex(
	"computed-explicit-destinations-by-service",
	func(ced *types.DecodedComputedExplicitDestinations) []*pbresource.Reference {
		if len(ced.Data.Destinations) == 0 {
			return nil
		}

		out := make([]*pbresource.Reference, 0, len(ced.Data.Destinations))
		for _, dest := range ced.Data.Destinations {
			if resource.EqualType(pbcatalog.ServiceType, dest.DestinationRef.Type) {
				out = append(out, dest.DestinationRef)
			}
		}
		return out
	},
)

// Cache: reverse CID => Service(destination)
var computedImplicitDestinationsByServiceIndex = indexers.RefOrIDIndex(
	"computed-implicit-destinations-by-service",
	func(ced *types.DecodedComputedImplicitDestinations) []*pbresource.Reference {
		if len(ced.Data.Destinations) == 0 {
			return nil
		}

		out := make([]*pbresource.Reference, 0, len(ced.Data.Destinations))
		for _, dest := range ced.Data.Destinations {
			if resource.EqualType(pbcatalog.ServiceType, dest.DestinationRef.Type) {
				out = append(out, dest.DestinationRef)
			}
		}
		return out
	},
)

// Cache: reverse Workload => WorkloadIdentity
var workloadByWorkloadIdentityIndex = indexers.RefOrIDIndex(
	"workload-by-workload-identity",
	func(wrk *types.DecodedWorkload) []*pbresource.Reference {
		if wrk.Data.Identity == "" {
			return nil
		}
		wid := &pbresource.Reference{
			Type:    pbauth.WorkloadIdentityType,
			Tenancy: wrk.GetId().GetTenancy(),
			Name:    wrk.Data.Identity,
		}
		return []*pbresource.Reference{wid}
	},
)

// Cache: reverse CR => SVC[backend]
var computedRoutesByBackendServiceIndex = meshindexes.ComputedRoutesByBackendServiceIndex()

// Cache: reverse SVC[*] => WI[*]
var serviceByWorkloadIdentityIndex = meshindexes.ServiceByWorkloadIdentityIndex()

func MapComputedImplicitDestinations(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	assertResourceType(pbmesh.ComputedImplicitDestinationsType, res.Id.Type)

	wiID := resource.ReplaceType(pbauth.WorkloadIdentityType, res.Id)

	workloads, err := rt.Cache.List(pbcatalog.WorkloadType, workloadByWorkloadIdentityIndex.Name(), wiID)
	if err != nil {
		return nil, err
	}

	return controller.MakeRequestsFromResources(pbmesh.ProxyStateTemplateType, workloads), nil
}

func MapComputedTrafficPermissions(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	assertResourceType(pbauth.ComputedTrafficPermissionsType, res.Id.Type)

	wiID := resource.ReplaceType(pbauth.WorkloadIdentityType, res.Id)

	workloads, err := rt.Cache.List(pbcatalog.WorkloadType, workloadByWorkloadIdentityIndex.Name(), wiID)
	if err != nil {
		return nil, err
	}

	return controller.MakeRequestsFromResources(pbmesh.ProxyStateTemplateType, workloads), nil
}

func MapComputedRoutes(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	assertResourceType(pbmesh.ComputedRoutesType, res.Id.Type)

	svcID := resource.ReplaceType(pbcatalog.ServiceType, res.Id)

	return mapServiceThroughDestinations(rt.Cache, svcID)
}

func MapService(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	assertResourceType(pbcatalog.ServiceType, res.Id.Type)

	pstIDs, err := mapServiceThroughDestinations(rt.Cache, res.Id)
	if err != nil {
		return nil, err
	}

	// Now walk the mesh configuration information backwards because
	// we need to find any PST that needs to DISCOVER endpoints for this
	// service as a part of mesh configuration and traffic routing.

	// Find all ComputedRoutes that reference this service.
	computedRoutes, err := rt.Cache.List(
		pbmesh.ComputedRoutesType,
		computedRoutesByBackendServiceIndex.Name(),
		res.Id,
	)
	if err != nil {
		return nil, err
	}

	for _, cr := range computedRoutes {
		// Find all Upstreams that reference a Service aligned with this
		// ComputedRoutes. Afterwards, find all Workloads selected by the
		// Upstreams, and align a PST with those.

		ids, err := MapComputedRoutes(ctx, rt, cr)
		if err != nil {
			return nil, err
		}
		pstIDs = append(pstIDs, ids...)
	}

	return pstIDs, nil
}

func mapServiceThroughDestinations(cache cache.ReadOnlyCache, svcID *pbresource.ID) ([]controller.Request, error) {
	explicitDests, err := cache.List(
		pbmesh.ComputedExplicitDestinationsType,
		computedExplicitDestinationsByServiceIndex.Name(),
		svcID,
	)
	if err != nil {
		return nil, err
	}

	implicitDests, err := cache.List(
		pbmesh.ComputedImplicitDestinationsType,
		computedImplicitDestinationsByServiceIndex.Name(),
		svcID,
	)
	if err != nil {
		return nil, err
	}

	// NOTE: ComputedExplicitDestinations are name-aligned with ProxyStateTemplates.
	// This is different for implicit dests, as ComputedImplicitDestinations is name-aligned with WorkloadIdentity
	reqs := make([]controller.Request, 0, len(explicitDests)+len(implicitDests))
	reqs = append(reqs, controller.MakeRequestsFromResources(
		pbmesh.ProxyStateTemplateType,
		explicitDests,
	)...)

	for _, dest := range implicitDests {
		wiID := resource.ReplaceType(pbauth.WorkloadIdentityType, dest.Id)

		workloads, err := cache.List(
			pbcatalog.WorkloadType,
			workloadByWorkloadIdentityIndex.Name(),
			wiID,
		)
		if err != nil {
			return nil, err
		}

		reqs = append(reqs, controller.MakeRequestsFromResources(
			pbmesh.ProxyStateTemplateType,
			workloads,
		)...)
	}

	return reqs, nil
}

func assertResourceType(expected, actual *pbresource.Type) {
	if !proto.Equal(expected, actual) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf("expected a query for a type of %q, you provided a type of %q", expected, actual))
	}
}
