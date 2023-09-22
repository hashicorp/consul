// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xroutemapper

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Mapper tracks the following relationships:
//
// - xRoute         <-> ParentRef Service
// - xRoute         <-> BackendRef Service
// - FailoverPolicy <-> DestRef Service
//
// It is the job of the controller, loader, and mapper to keep the mappings up
// to date whenever new data is loaded. Notably because the dep mapper events
// do not signal when data is deleted, it is the job of the reconcile load of
// the data causing the event to notice something has been deleted and to
// untrack it here.
type Mapper struct {
	boundRefMapper *bimapper.Mapper

	httpRouteParentMapper *bimapper.Mapper
	grpcRouteParentMapper *bimapper.Mapper
	tcpRouteParentMapper  *bimapper.Mapper

	httpRouteBackendMapper *bimapper.Mapper
	grpcRouteBackendMapper *bimapper.Mapper
	tcpRouteBackendMapper  *bimapper.Mapper

	failMapper catalog.FailoverPolicyMapper
}

// New creates a new Mapper.
func New() *Mapper {
	return &Mapper{
		boundRefMapper: bimapper.NewWithWildcardLinkType(pbmesh.ComputedRoutesType),

		httpRouteParentMapper: bimapper.New(pbmesh.HTTPRouteType, pbcatalog.ServiceType),
		grpcRouteParentMapper: bimapper.New(pbmesh.GRPCRouteType, pbcatalog.ServiceType),
		tcpRouteParentMapper:  bimapper.New(pbmesh.TCPRouteType, pbcatalog.ServiceType),

		httpRouteBackendMapper: bimapper.New(pbmesh.HTTPRouteType, pbcatalog.ServiceType),
		grpcRouteBackendMapper: bimapper.New(pbmesh.GRPCRouteType, pbcatalog.ServiceType),
		tcpRouteBackendMapper:  bimapper.New(pbmesh.TCPRouteType, pbcatalog.ServiceType),

		failMapper: catalog.NewFailoverPolicyMapper(),
	}
}

func (m *Mapper) getRouteBiMappers(typ *pbresource.Type) (parent, backend *bimapper.Mapper) {
	switch {
	case resource.EqualType(pbmesh.HTTPRouteType, typ):
		return m.httpRouteParentMapper, m.httpRouteBackendMapper
	case resource.EqualType(pbmesh.GRPCRouteType, typ):
		return m.grpcRouteParentMapper, m.grpcRouteBackendMapper
	case resource.EqualType(pbmesh.TCPRouteType, typ):
		return m.tcpRouteParentMapper, m.tcpRouteBackendMapper
	default:
		panic("unknown xroute type: " + resource.TypeToString(typ))
	}
}

func (m *Mapper) walkRouteParentBiMappers(fn func(bm *bimapper.Mapper)) {
	for _, bm := range []*bimapper.Mapper{
		m.httpRouteParentMapper,
		m.grpcRouteParentMapper,
		m.tcpRouteParentMapper,
	} {
		fn(bm)
	}
}

func (m *Mapper) walkRouteBackendBiMappers(fn func(bm *bimapper.Mapper)) {
	for _, bm := range []*bimapper.Mapper{
		m.httpRouteBackendMapper,
		m.grpcRouteBackendMapper,
		m.tcpRouteBackendMapper,
	} {
		fn(bm)
	}
}

func (m *Mapper) TrackComputedRoutes(cr *types.DecodedComputedRoutes) {
	if cr != nil {
		refs := refSliceToRefSlice(cr.Data.BoundReferences)
		m.boundRefMapper.TrackItem(cr.Resource.Id, refs)
	}
}

func (m *Mapper) UntrackComputedRoutes(id *pbresource.ID) {
	m.boundRefMapper.UntrackItem(id)
}

// TrackXRoute indexes the xRoute->parentRefService and
// xRoute->backendRefService relationship.
func (m *Mapper) TrackXRoute(id *pbresource.ID, xroute types.XRouteData) {
	parent, backend := m.getRouteBiMappers(id.Type)
	if parent == nil || backend == nil {
		return
	}

	parentRefs := parentRefSliceToRefSlice(xroute.GetParentRefs())
	backendRefs := backendRefSliceToRefSlice(xroute.GetUnderlyingBackendRefs())

	parent.TrackItem(id, parentRefs)
	backend.TrackItem(id, backendRefs)
}

// UntrackXRoute undoes TrackXRoute.
func (m *Mapper) UntrackXRoute(id *pbresource.ID) {
	parent, backend := m.getRouteBiMappers(id.Type)
	if parent == nil || backend == nil {
		return
	}

	parent.UntrackItem(id)
	backend.UntrackItem(id)
}

// RouteIDsByParentServiceRef returns xRoute IDs that have a direct parentRef link to
// the provided service.
func (m *Mapper) RouteIDsByParentServiceRef(ref *pbresource.Reference) []*pbresource.ID {
	var out []*pbresource.ID
	m.walkRouteParentBiMappers(func(bm *bimapper.Mapper) {
		got := bm.ItemsForLink(resource.IDFromReference(ref))
		out = append(out, got...)
	})
	return out
}

// RouteIDsByBackendServiceRef returns xRoute IDs that have a direct backendRef
// link to the provided service.
func (m *Mapper) RouteIDsByBackendServiceRef(ref *pbresource.Reference) []*pbresource.ID {
	var out []*pbresource.ID
	m.walkRouteBackendBiMappers(func(bm *bimapper.Mapper) {
		got := bm.ItemsForLink(resource.IDFromReference(ref))
		out = append(out, got...)
	})
	return out
}

// ParentServiceRefsByRouteID is the opposite of RouteIDsByParentServiceRef.
func (m *Mapper) ParentServiceRefsByRouteID(item *pbresource.ID) []*pbresource.Reference {
	parent, _ := m.getRouteBiMappers(item.Type)
	if parent == nil {
		return nil
	}
	return parent.LinksForItem(item)
}

// BackendServiceRefsByRouteID is the opposite of RouteIDsByBackendServiceRef.
func (m *Mapper) BackendServiceRefsByRouteID(item *pbresource.ID) []*pbresource.Reference {
	_, backend := m.getRouteBiMappers(item.Type)
	if backend == nil {
		return nil
	}
	return backend.LinksForItem(item)
}

// MapHTTPRoute will map HTTPRoute changes to ComputedRoutes changes.
func (m *Mapper) MapHTTPRoute(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	return mapXRouteToComputedRoutes[*pbmesh.HTTPRoute](res, m)
}

// MapGRPCRoute will map GRPCRoute changes to ComputedRoutes changes.
func (m *Mapper) MapGRPCRoute(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	return mapXRouteToComputedRoutes[*pbmesh.GRPCRoute](res, m)
}

// MapTCPRoute will map TCPRoute changes to ComputedRoutes changes.
func (m *Mapper) MapTCPRoute(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	return mapXRouteToComputedRoutes[*pbmesh.TCPRoute](res, m)
}

// mapXRouteToComputedRoutes will map xRoute changes to ComputedRoutes changes.
func mapXRouteToComputedRoutes[T types.XRouteData](res *pbresource.Resource, m *Mapper) ([]controller.Request, error) {
	dec, err := resource.Decode[T](res)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling xRoute: %w", err)
	}

	route := dec.Data

	m.TrackXRoute(res.Id, route)

	refs := parentRefSliceToRefSlice(route.GetParentRefs())

	// Augment with any bound refs to cover the case where an xRoute used to
	// have a parentRef to a service and now no longer does.
	prevRefs := m.boundRefMapper.ItemRefsForLink(dec.Resource.Id)
	for _, ref := range prevRefs {
		refs = append(refs, ref)
	}

	return controller.MakeRequests(pbmesh.ComputedRoutesType, refs), nil
}

func (m *Mapper) MapFailoverPolicy(
	_ context.Context,
	_ controller.Runtime,
	res *pbresource.Resource,
) ([]controller.Request, error) {
	if !types.IsFailoverPolicyType(res.Id.Type) {
		return nil, fmt.Errorf("type is not a failover policy type: %s", res.Id.Type)
	}

	dec, err := resource.Decode[*pbcatalog.FailoverPolicy](res)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling failover policy: %w", err)
	}

	m.failMapper.TrackFailover(dec)

	// Since this is name-aligned, just switch the type and find routes that
	// will route any traffic to this destination service.
	svcID := resource.ReplaceType(pbcatalog.ServiceType, res.Id)

	return m.mapXRouteDirectServiceRefToComputedRoutesByID(svcID)
}

func (m *Mapper) TrackFailoverPolicy(failover *types.DecodedFailoverPolicy) {
	if failover != nil {
		m.failMapper.TrackFailover(failover)
	}
}

func (m *Mapper) UntrackFailoverPolicy(failoverPolicyID *pbresource.ID) {
	m.failMapper.UntrackFailover(failoverPolicyID)
}

func (m *Mapper) MapDestinationPolicy(
	_ context.Context,
	_ controller.Runtime,
	res *pbresource.Resource,
) ([]controller.Request, error) {
	if !types.IsDestinationPolicyType(res.Id.Type) {
		return nil, fmt.Errorf("type is not a destination policy type: %s", res.Id.Type)
	}

	// Since this is name-aligned, just switch the type and find routes that
	// will route any traffic to this destination service.
	svcID := resource.ReplaceType(pbcatalog.ServiceType, res.Id)

	return m.mapXRouteDirectServiceRefToComputedRoutesByID(svcID)
}

func (m *Mapper) MapService(
	_ context.Context,
	_ controller.Runtime,
	res *pbresource.Resource,
) ([]controller.Request, error) {
	// Ultimately we want to wake up a ComputedRoutes if either of the
	// following exist:
	//
	// 1. xRoute[parentRef=OUTPUT_EVENT; backendRef=INPUT_EVENT]
	// 2. xRoute[parentRef=OUTPUT_EVENT; backendRef=SOMETHING], FailoverPolicy[name=SOMETHING, destRef=INPUT_EVENT]

	// (case 2) First find all failover policies that have a reference to our input service.
	failPolicyIDs := m.failMapper.FailoverIDsByService(res.Id)
	effectiveServiceIDs := sliceReplaceType(failPolicyIDs, pbcatalog.ServiceType)

	// (case 1) Do the direct mapping also.
	effectiveServiceIDs = append(effectiveServiceIDs, res.Id)

	var reqs []controller.Request
	for _, svcID := range effectiveServiceIDs {
		got, err := m.mapXRouteDirectServiceRefToComputedRoutesByID(svcID)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, got...)
	}

	return reqs, nil
}

// NOTE: this function does not interrogate down into failover policies
func (m *Mapper) mapXRouteDirectServiceRefToComputedRoutesByID(svcID *pbresource.ID) ([]controller.Request, error) {
	if !types.IsServiceType(svcID.Type) {
		return nil, fmt.Errorf("type is not a service type: %s", svcID.Type)
	}

	// return 1 hit for the name aligned mesh config
	primaryReq := controller.Request{
		ID: resource.ReplaceType(pbmesh.ComputedRoutesType, svcID),
	}

	svcRef := resource.Reference(svcID, "")

	// Find all routes with an explicit backend ref to this service.
	//
	// the "name aligned" inclusion above should handle the implicit default
	// destination implied by a parent ref without us having to do much more.
	routeIDs := m.RouteIDsByBackendServiceRef(svcRef)

	out := make([]controller.Request, 0, 1+len(routeIDs)) // estimated
	out = append(out, primaryReq)

	for _, routeID := range routeIDs {
		// Find all parent refs of this route.
		svcRefs := m.ParentServiceRefsByRouteID(routeID)

		out = append(out, controller.MakeRequests(
			pbmesh.ComputedRoutesType,
			svcRefs,
		)...)
	}

	return out, nil
}
