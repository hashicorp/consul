// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestLoadResourcesForComputedRoutes(t *testing.T) {
	rclient := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		Run(t)

	ctl := controller.NewTestController(Controller(), rclient).
		WithLogger(testutil.Logger(t))
	rt := ctl.Runtime()
	client := rtest.NewClient(rt.Client)

	tenancy := resource.DefaultNamespacedTenancy()

	writeHTTP := func(name string, data *pbmesh.HTTPRoute) *types.DecodedHTTPRoute {
		res := rtest.Resource(pbmesh.HTTPRouteType, name).
			WithTenancy(tenancy).
			WithData(t, data).
			Write(t, client)
		dec, err := resource.Decode[*pbmesh.HTTPRoute](res)
		require.NoError(t, err)
		return dec
	}

	writeGRPC := func(name string, data *pbmesh.GRPCRoute) *types.DecodedGRPCRoute {
		res := rtest.Resource(pbmesh.GRPCRouteType, name).
			WithTenancy(tenancy).
			WithData(t, data).
			Write(t, client)
		dec, err := resource.Decode[*pbmesh.GRPCRoute](res)
		require.NoError(t, err)
		return dec
	}

	writeTCP := func(name string, data *pbmesh.TCPRoute) *types.DecodedTCPRoute {
		res := rtest.Resource(pbmesh.TCPRouteType, name).
			WithTenancy(tenancy).
			WithData(t, data).
			Write(t, client)
		dec, err := resource.Decode[*pbmesh.TCPRoute](res)
		require.NoError(t, err)
		return dec
	}

	writeDestPolicy := func(name string, data *pbmesh.DestinationPolicy) *types.DecodedDestinationPolicy {
		res := rtest.Resource(pbmesh.DestinationPolicyType, name).
			WithTenancy(tenancy).
			WithData(t, data).
			Write(t, client)
		dec, err := resource.Decode[*pbmesh.DestinationPolicy](res)
		require.NoError(t, err)
		return dec
	}

	writeFailover := func(name string, data *pbcatalog.ComputedFailoverPolicy) *types.DecodedComputedFailoverPolicy {
		res := rtest.Resource(pbcatalog.ComputedFailoverPolicyType, name).
			WithTenancy(tenancy).
			WithData(t, data).
			Write(t, client)
		dec, err := resource.Decode[*pbcatalog.ComputedFailoverPolicy](res)
		require.NoError(t, err)
		return dec
	}

	writeService := func(name string, data *pbcatalog.Service) *types.DecodedService {
		res := rtest.Resource(pbcatalog.ServiceType, name).
			WithTenancy(tenancy).
			WithData(t, data).
			Write(t, client)
		dec, err := resource.Decode[*pbcatalog.Service](res)
		require.NoError(t, err)
		return dec
	}

	/////////////////////////////////////

	// Init some port-aligned services.
	apiSvc := writeService("api", &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	})
	adminSvc := writeService("admin", &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	})
	fooSvc := writeService("foo", &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	})
	barSvc := writeService("bar", &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	})

	apiRoutesID := &pbresource.ID{
		Type:    pbmesh.ComputedRoutesType,
		Tenancy: tenancy,
		Name:    "api",
	}
	adminRoutesID := &pbresource.ID{
		Type:    pbmesh.ComputedRoutesType,
		Tenancy: tenancy,
		Name:    "admin",
	}

	testutil.RunStep(t, "only service", func(t *testing.T) {
		out, err := LoadResourcesForComputedRoutes(rt.Cache, apiRoutesID)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, NewRelatedResources().AddResources(
			apiSvc,
		).AddComputedRoutesIDs(apiRoutesID), out)
		require.Equal(t, doubleMap(t /* empty */), out.RoutesByParentRef)
	})

	// Write one silly http route
	route1 := writeHTTP("api-route1", &pbmesh.HTTPRoute{
		ParentRefs: []*pbmesh.ParentReference{{
			Ref: newRef(pbcatalog.ServiceType, "api", tenancy),
			// all ports
		}},
	})

	testutil.RunStep(t, "one silly route", func(t *testing.T) {
		out, err := LoadResourcesForComputedRoutes(rt.Cache, apiRoutesID)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, NewRelatedResources().AddResources(
			apiSvc,
			route1,
		).AddComputedRoutesIDs(apiRoutesID), out)
		require.Equal(t, doubleMap(t,
			apiSvc, route1,
		), out.RoutesByParentRef)
	})

	// add a second route that is more interesting and is TCP
	route2 := writeTCP("api-route2", &pbmesh.TCPRoute{
		ParentRefs: []*pbmesh.ParentReference{{
			Ref: newRef(pbcatalog.ServiceType, "api", tenancy),
			// all ports
		}},
		Rules: []*pbmesh.TCPRouteRule{{
			BackendRefs: []*pbmesh.TCPBackendRef{
				{
					BackendRef: &pbmesh.BackendReference{
						Ref: newRef(pbcatalog.ServiceType, "foo", tenancy),
					},
					Weight: 30,
				},
				{
					BackendRef: &pbmesh.BackendReference{
						Ref: newRef(pbcatalog.ServiceType, "bar", tenancy),
					},
					Weight: 70,
				},
			},
		}},
	})

	testutil.RunStep(t, "two routes", func(t *testing.T) {
		out, err := LoadResourcesForComputedRoutes(rt.Cache, apiRoutesID)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, NewRelatedResources().AddResources(
			apiSvc,
			fooSvc,
			barSvc,
			route1,
			route2,
		).AddComputedRoutesIDs(apiRoutesID), out)
		require.Equal(t, doubleMap(t,
			apiSvc, route1,
			apiSvc, route2,
		), out.RoutesByParentRef)
	})

	// update the first to overlap with the second
	route1 = writeHTTP("api-route1", &pbmesh.HTTPRoute{
		ParentRefs: []*pbmesh.ParentReference{
			{
				Ref: newRef(pbcatalog.ServiceType, "api", tenancy),
				// all ports
			},
			{
				Ref: newRef(pbcatalog.ServiceType, "admin", tenancy),
				// all ports
			},
		},
	})

	testutil.RunStep(t, "two overlapping computed routes resources", func(t *testing.T) {
		out, err := LoadResourcesForComputedRoutes(rt.Cache, apiRoutesID)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, NewRelatedResources().AddResources(
			apiSvc,
			fooSvc,
			barSvc,
			adminSvc,
			route1,
			route2,
		).AddComputedRoutesIDs(apiRoutesID, adminRoutesID), out)
		require.Equal(t, doubleMap(t,
			apiSvc, route1,
			apiSvc, route2,
			adminSvc, route1,
		), out.RoutesByParentRef)
	})

	// add a third (GRPC) that overlaps them both

	route3 := writeGRPC("api-route3", &pbmesh.GRPCRoute{
		ParentRefs: []*pbmesh.ParentReference{
			{
				Ref: newRef(pbcatalog.ServiceType, "api", tenancy),
				// all ports
			},
			{
				Ref: newRef(pbcatalog.ServiceType, "admin", tenancy),
				// all ports
			},
		},
	})

	testutil.RunStep(t, "three overlapping computed routes resources", func(t *testing.T) {
		out, err := LoadResourcesForComputedRoutes(rt.Cache, apiRoutesID)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, NewRelatedResources().AddResources(
			apiSvc,
			fooSvc,
			barSvc,
			adminSvc,
			route1,
			route2,
			route3,
		).AddComputedRoutesIDs(apiRoutesID, adminRoutesID), out)
		require.Equal(t, doubleMap(t,
			apiSvc, route1,
			apiSvc, route2,
			apiSvc, route3,
			adminSvc, route1,
			adminSvc, route3,
		), out.RoutesByParentRef)
	})

	// We untrack the first and third.
	client.MustDelete(t, route1.Resource.Id)
	client.MustDelete(t, route3.Resource.Id)

	testutil.RunStep(t, "delete first and third route", func(t *testing.T) {
		out, err := LoadResourcesForComputedRoutes(rt.Cache, apiRoutesID)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, NewRelatedResources().AddResources(
			apiSvc,
			fooSvc,
			barSvc,
			route2,
		).AddComputedRoutesIDs(apiRoutesID), out)
		require.Equal(t, doubleMap(t,
			apiSvc, route2,
		), out.RoutesByParentRef)
	})

	barFailover := writeFailover("bar", &pbcatalog.ComputedFailoverPolicy{
		PortConfigs: map[string]*pbcatalog.FailoverConfig{
			"http": {
				Destinations: []*pbcatalog.FailoverDestination{
					{
						Ref:  newRef(pbcatalog.ServiceType, "admin", tenancy),
						Port: "http",
					},
				},
			},
		},
		BoundReferences: []*pbresource.Reference{newRef(pbcatalog.ServiceType, "admin", tenancy)},
	})

	testutil.RunStep(t, "add a failover", func(t *testing.T) {
		out, err := LoadResourcesForComputedRoutes(rt.Cache, apiRoutesID)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, NewRelatedResources().AddResources(
			apiSvc,
			fooSvc,
			barSvc,
			adminSvc,
			route2,
			barFailover,
		).AddComputedRoutesIDs(apiRoutesID), out)
		require.Equal(t, doubleMap(t,
			apiSvc, route2,
		), out.RoutesByParentRef)
	})

	fooDestPolicy := writeDestPolicy("foo", &pbmesh.DestinationPolicy{
		PortConfigs: map[string]*pbmesh.DestinationConfig{
			"www": {
				ConnectTimeout: durationpb.New(55 * time.Second),
			},
		},
	})
	adminDestPolicy := writeDestPolicy("admin", &pbmesh.DestinationPolicy{
		PortConfigs: map[string]*pbmesh.DestinationConfig{
			"http": {
				ConnectTimeout: durationpb.New(222 * time.Second),
			},
		},
	})

	testutil.RunStep(t, "add a dest policy", func(t *testing.T) {
		out, err := LoadResourcesForComputedRoutes(rt.Cache, apiRoutesID)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, NewRelatedResources().AddResources(
			apiSvc,
			fooSvc,
			barSvc,
			adminSvc,
			route2,
			barFailover,
			fooDestPolicy,
			adminDestPolicy, // adminDestPolicy shows up indirectly via a FailoverPolicy
		).AddComputedRoutesIDs(apiRoutesID), out)
		require.Equal(t, doubleMap(t,
			apiSvc, route2,
		), out.RoutesByParentRef)
	})
}

type resourceGetter interface {
	GetResource() *pbresource.Resource
}

func doubleMap(t *testing.T, list ...resourceGetter) map[resource.ReferenceKey]map[resource.ReferenceKey]struct{} {
	if len(list)%2 != 0 {
		t.Fatalf("list must have an even number of references")
	}
	out := make(map[resource.ReferenceKey]map[resource.ReferenceKey]struct{})
	for i := 0; i < len(list); i += 2 {
		svcRK := resource.NewReferenceKey(list[i].GetResource().Id)
		routeRK := resource.NewReferenceKey(list[i+1].GetResource().Id)

		m, ok := out[svcRK]
		if !ok {
			m = make(map[resource.ReferenceKey]struct{})
			out[svcRK] = m
		}
		m[routeRK] = struct{}{}
	}
	return out
}
