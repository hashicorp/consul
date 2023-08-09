// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"fmt"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// RelatedResources is a spiritual successor of *configentry.DiscoveryChainSet
type RelatedResources struct {
	ComputedRoutesList []*pbresource.ID
	// RoutesByParentRef is a map of a parent Service to the xRoutes that compose it.
	RoutesByParentRef   map[resource.ReferenceKey]map[resource.ReferenceKey]struct{}
	HTTPRoutes          map[resource.ReferenceKey]*types.DecodedHTTPRoute
	GRPCRoutes          map[resource.ReferenceKey]*types.DecodedGRPCRoute
	TCPRoutes           map[resource.ReferenceKey]*types.DecodedTCPRoute
	Services            map[resource.ReferenceKey]*types.DecodedService
	FailoverPolicies    map[resource.ReferenceKey]*types.DecodedFailoverPolicy
	DestinationPolicies map[resource.ReferenceKey]*types.DecodedDestinationPolicy
}

func NewRelatedResources() *RelatedResources {
	return &RelatedResources{
		RoutesByParentRef:   make(map[resource.ReferenceKey]map[resource.ReferenceKey]struct{}),
		HTTPRoutes:          make(map[resource.ReferenceKey]*types.DecodedHTTPRoute),
		GRPCRoutes:          make(map[resource.ReferenceKey]*types.DecodedGRPCRoute),
		TCPRoutes:           make(map[resource.ReferenceKey]*types.DecodedTCPRoute),
		Services:            make(map[resource.ReferenceKey]*types.DecodedService),
		FailoverPolicies:    make(map[resource.ReferenceKey]*types.DecodedFailoverPolicy),
		DestinationPolicies: make(map[resource.ReferenceKey]*types.DecodedDestinationPolicy),
	}
}

func (r *RelatedResources) AddComputedRoutesIDs(list ...*pbresource.ID) *RelatedResources {
	for _, id := range list {
		r.AddComputedRoutesID(id)
	}
	return r
}

func (r *RelatedResources) AddComputedRoutesID(id *pbresource.ID) {
	if !resource.EqualType(id.Type, types.ComputedRoutesType) {
		panic(fmt.Sprintf("expected *mesh.ComputedRoutes, not %s", resource.TypeToString(id.Type)))
	}
	r.ComputedRoutesList = append(r.ComputedRoutesList, id)
}

func (r *RelatedResources) AddResources(list ...decodedResource) *RelatedResources {
	for _, res := range list {
		_ = r.AddResource(res)
	}
	return r
}

type decodedResource interface {
	GetResource() *pbresource.Resource
}

func (r *RelatedResources) AddResource(res decodedResource) bool {
	if res == nil {
		return false
	}

	switch dec := res.(type) {
	case *types.DecodedHTTPRoute:
		r.addRouteSetEntries(dec.Resource, dec.Data)
		return addResource(dec.Resource.Id, dec, r.HTTPRoutes)
	case *types.DecodedGRPCRoute:
		r.addRouteSetEntries(dec.Resource, dec.Data)
		return addResource(dec.Resource.Id, dec, r.GRPCRoutes)
	case *types.DecodedTCPRoute:
		r.addRouteSetEntries(dec.Resource, dec.Data)
		return addResource(dec.Resource.Id, dec, r.TCPRoutes)
	case *types.DecodedService:
		return addResource(dec.Resource.Id, dec, r.Services)
	case *types.DecodedFailoverPolicy:
		return addResource(dec.Resource.Id, dec, r.FailoverPolicies)
	case *types.DecodedDestinationPolicy:
		return addResource(dec.Resource.Id, dec, r.DestinationPolicies)
	default:
		panic(fmt.Sprintf("unknown decoded resource type: %T", res))
	}
}

func (r *RelatedResources) addRouteSetEntries(
	res *pbresource.Resource,
	xroute types.XRouteData,
) {
	if res == nil || xroute == nil {
		return
	}

	routeRK := resource.NewReferenceKey(res.Id)

	for _, parentRef := range xroute.GetParentRefs() {
		if parentRef.Ref == nil || !types.IsServiceType(parentRef.Ref.Type) {
			continue
		}
		svcRK := resource.NewReferenceKey(parentRef.Ref)

		r.addRouteByParentRef(svcRK, routeRK)
	}
}

func (r *RelatedResources) addRouteByParentRef(svcRK, xRouteRK resource.ReferenceKey) {
	m, ok := r.RoutesByParentRef[svcRK]
	if !ok {
		m = make(map[resource.ReferenceKey]struct{})
		r.RoutesByParentRef[svcRK] = m
	}
	m[xRouteRK] = struct{}{}
}

type RouteWalkFunc func(
	rk resource.ReferenceKey,
	res *pbresource.Resource,
	route types.XRouteData,
)

func (r *RelatedResources) WalkRoutes(fn RouteWalkFunc) {
	for rk, route := range r.HTTPRoutes {
		fn(rk, route.Resource, route.Data)
	}
	for rk, route := range r.GRPCRoutes {
		fn(rk, route.Resource, route.Data)
	}
	for rk, route := range r.TCPRoutes {
		fn(rk, route.Resource, route.Data)
	}
}

func (r *RelatedResources) WalkRoutesForParentRef(parentRef *pbresource.Reference, fn RouteWalkFunc) {
	if !resource.EqualType(parentRef.Type, catalog.ServiceType) {
		panic(fmt.Sprintf("expected *catalog.Service, not %s", resource.TypeToString(parentRef.Type)))
	}
	routeMap := r.RoutesByParentRef[resource.NewReferenceKey(parentRef)]
	if len(routeMap) == 0 {
		return
	}

	for rk := range routeMap {
		if route, ok := r.HTTPRoutes[rk]; ok {
			fn(rk, route.Resource, route.Data)
			continue
		}
		if route, ok := r.GRPCRoutes[rk]; ok {
			fn(rk, route.Resource, route.Data)
			continue
		}
		if route, ok := r.TCPRoutes[rk]; ok {
			fn(rk, route.Resource, route.Data)
			continue
		}
	}
}

func (r *RelatedResources) GetService(ref resource.ReferenceOrID) *types.DecodedService {
	return r.Services[resource.NewReferenceKey(ref)]
}

func (r *RelatedResources) GetFailoverPolicy(ref resource.ReferenceOrID) *types.DecodedFailoverPolicy {
	return r.FailoverPolicies[resource.NewReferenceKey(ref)]
}

func (r *RelatedResources) GetFailoverPolicyForService(ref resource.ReferenceOrID) *types.DecodedFailoverPolicy {
	failRef := &pbresource.Reference{
		Type:    catalog.FailoverPolicyType,
		Tenancy: ref.GetTenancy(),
		Name:    ref.GetName(),
	}
	return r.GetFailoverPolicy(failRef)
}

func (r *RelatedResources) GetDestinationPolicy(ref resource.ReferenceOrID) *types.DecodedDestinationPolicy {
	return r.DestinationPolicies[resource.NewReferenceKey(ref)]
}

func (r *RelatedResources) GetDestinationPolicyForService(ref resource.ReferenceOrID) *types.DecodedDestinationPolicy {
	destRef := &pbresource.Reference{
		Type:    types.DestinationPolicyType,
		Tenancy: ref.GetTenancy(),
		Name:    ref.GetName(),
	}
	return r.GetDestinationPolicy(destRef)
}

func addResource[V any](id *pbresource.ID, res *V, m map[resource.ReferenceKey]*V) bool {
	if res == nil {
		return false
	}

	rk := resource.NewReferenceKey(id)
	if _, ok := m[rk]; ok {
		return false
	}
	m[rk] = res
	return true
}
