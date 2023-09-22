// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xroutemapper

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMapper_HTTPRoute_Tracking(t *testing.T) {
	testMapper_Tracking(t, pbmesh.HTTPRouteType, func(t *testing.T, parentRefs []*pbmesh.ParentReference, backendRefs []*pbmesh.BackendReference) proto.Message {
		route := &pbmesh.HTTPRoute{
			ParentRefs: parentRefs,
		}
		for _, backendRef := range backendRefs {
			route.Rules = append(route.Rules, &pbmesh.HTTPRouteRule{
				BackendRefs: []*pbmesh.HTTPBackendRef{
					{BackendRef: backendRef},
				},
			})
		}
		return route
	})
}

func TestMapper_GRPCRoute_Tracking(t *testing.T) {
	testMapper_Tracking(t, pbmesh.GRPCRouteType, func(t *testing.T, parentRefs []*pbmesh.ParentReference, backendRefs []*pbmesh.BackendReference) proto.Message {
		route := &pbmesh.GRPCRoute{
			ParentRefs: parentRefs,
		}
		for _, backendRef := range backendRefs {
			route.Rules = append(route.Rules, &pbmesh.GRPCRouteRule{
				BackendRefs: []*pbmesh.GRPCBackendRef{
					{BackendRef: backendRef},
				},
			})
		}
		return route
	})
}

func TestMapper_TCPRoute_Tracking(t *testing.T) {
	testMapper_Tracking(t, pbmesh.TCPRouteType, func(t *testing.T, parentRefs []*pbmesh.ParentReference, backendRefs []*pbmesh.BackendReference) proto.Message {
		route := &pbmesh.TCPRoute{
			ParentRefs: parentRefs,
		}
		for _, backendRef := range backendRefs {
			route.Rules = append(route.Rules, &pbmesh.TCPRouteRule{
				BackendRefs: []*pbmesh.TCPBackendRef{
					{BackendRef: backendRef},
				},
			})
		}
		return route
	})
}

func testMapper_Tracking(t *testing.T, typ *pbresource.Type, newRoute func(t *testing.T, parentRefs []*pbmesh.ParentReference, backendRefs []*pbmesh.BackendReference) proto.Message) {
	registry := resource.NewRegistry()
	types.Register(registry)
	catalog.RegisterTypes(registry)

	newService := func(name string) *pbresource.Resource {
		svc := rtest.Resource(pbcatalog.ServiceType, name).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, &pbcatalog.Service{}).
			Build()
		rtest.ValidateAndNormalize(t, registry, svc)
		return svc
	}

	newDestPolicy := func(name string, dur time.Duration) *pbresource.Resource {
		policy := rtest.Resource(pbmesh.DestinationPolicyType, name).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(dur),
					},
				},
			}).Build()
		rtest.ValidateAndNormalize(t, registry, policy)
		return policy
	}

	newFailPolicy := func(name string, refs ...*pbresource.Reference) *pbresource.Resource {
		var dests []*pbcatalog.FailoverDestination
		for _, ref := range refs {
			dests = append(dests, &pbcatalog.FailoverDestination{
				Ref: ref,
			})
		}
		policy := rtest.Resource(pbcatalog.FailoverPolicyType, name).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: dests,
				},
			}).Build()
		rtest.ValidateAndNormalize(t, registry, policy)
		return policy
	}

	apiComputedRoutes := newID(pbmesh.ComputedRoutesType, "api")
	wwwComputedRoutes := newID(pbmesh.ComputedRoutesType, "www")
	barComputedRoutes := newID(pbmesh.ComputedRoutesType, "bar")
	fooComputedRoutes := newID(pbmesh.ComputedRoutesType, "foo")
	zimComputedRoutes := newID(pbmesh.ComputedRoutesType, "zim")
	girComputedRoutes := newID(pbmesh.ComputedRoutesType, "gir")

	m := New()

	var (
		apiSvc = newService("api")
		wwwSvc = newService("www")
		barSvc = newService("bar")
		fooSvc = newService("foo")
		zimSvc = newService("zim")
		girSvc = newService("gir")

		apiSvcRef = resource.Reference(apiSvc.Id, "")
		wwwSvcRef = resource.Reference(wwwSvc.Id, "")
		barSvcRef = resource.Reference(barSvc.Id, "")
		fooSvcRef = resource.Reference(fooSvc.Id, "")
		zimSvcRef = resource.Reference(zimSvc.Id, "")
		girSvcRef = resource.Reference(girSvc.Id, "")

		apiDest = newDestPolicy("api", 55*time.Second)
		wwwDest = newDestPolicy("www", 123*time.Second)

		// Start out easy and don't have failover policies that reference other services.
		apiFail = newFailPolicy("api", newRef(pbcatalog.ServiceType, "api"))
		wwwFail = newFailPolicy("www", newRef(pbcatalog.ServiceType, "www"))
		barFail = newFailPolicy("bar", newRef(pbcatalog.ServiceType, "bar"))
	)

	testutil.RunStep(t, "only name aligned defaults", func(t *testing.T) {
		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes)
		requireTracking(t, m, fooSvc, fooComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes)

		// This will track the failover policies.
		requireTracking(t, m, apiFail, apiComputedRoutes)
		requireTracking(t, m, wwwFail, wwwComputedRoutes)
		requireTracking(t, m, barFail, barComputedRoutes)

		// verify other helper methods
		for _, ref := range []*pbresource.Reference{apiSvcRef, wwwSvcRef, barSvcRef, fooSvcRef, zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	var (
		route1 *pbresource.Resource
	)
	testutil.RunStep(t, "track a name-aligned xroute", func(t *testing.T) {
		// First route will also not cross any services.
		route1 := rtest.Resource(typ, "route-1").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, newRoute(t,
				[]*pbmesh.ParentReference{
					{Ref: newRef(pbcatalog.ServiceType, "api")},
				},
				[]*pbmesh.BackendReference{
					newBackendRef("api"),
				},
			)).Build()
		rtest.ValidateAndNormalize(t, registry, route1)

		requireTracking(t, m, route1, apiComputedRoutes)

		// Now 'api' references should trigger more, but be duplicate-suppressed.
		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes)
		requireTracking(t, m, fooSvc, fooComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes)

		requireTracking(t, m, apiFail, apiComputedRoutes)
		requireTracking(t, m, wwwFail, wwwComputedRoutes)
		requireTracking(t, m, barFail, barComputedRoutes)

		// verify other helper methods
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef}, m.BackendServiceRefsByRouteID(route1.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef}, m.ParentServiceRefsByRouteID(route1.Id))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id}, m.RouteIDsByBackendServiceRef(apiSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id}, m.RouteIDsByParentServiceRef(apiSvcRef))

		for _, ref := range []*pbresource.Reference{wwwSvcRef, barSvcRef, fooSvcRef, zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	testutil.RunStep(t, "make the route cross services", func(t *testing.T) {
		route1 = rtest.Resource(typ, "route-1").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, newRoute(t,
				[]*pbmesh.ParentReference{
					{Ref: newRef(pbcatalog.ServiceType, "api")},
				},
				[]*pbmesh.BackendReference{
					newBackendRef("www"),
				},
			)).Build()
		rtest.ValidateAndNormalize(t, registry, route1)

		// Now witness the update.
		requireTracking(t, m, route1, apiComputedRoutes)

		// Now 'api' references should trigger different things.
		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes)
		requireTracking(t, m, fooSvc, fooComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes, apiComputedRoutes)

		requireTracking(t, m, apiFail, apiComputedRoutes)
		requireTracking(t, m, wwwFail, wwwComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, barFail, barComputedRoutes)

		// verify other helper methods
		prototest.AssertElementsMatch(t, []*pbresource.Reference{wwwSvcRef}, m.BackendServiceRefsByRouteID(route1.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef}, m.ParentServiceRefsByRouteID(route1.Id))

		require.Empty(t, m.RouteIDsByBackendServiceRef(apiSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id}, m.RouteIDsByParentServiceRef(apiSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id}, m.RouteIDsByBackendServiceRef(wwwSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(wwwSvcRef))

		for _, ref := range []*pbresource.Reference{barSvcRef, fooSvcRef, zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	var (
		route2 *pbresource.Resource
	)
	testutil.RunStep(t, "make another route sharing a parent with the first", func(t *testing.T) {
		route2 = rtest.Resource(typ, "route-2").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, newRoute(t,
				[]*pbmesh.ParentReference{
					{Ref: newRef(pbcatalog.ServiceType, "api")},
					{Ref: newRef(pbcatalog.ServiceType, "foo")},
				},
				[]*pbmesh.BackendReference{
					newBackendRef("bar"),
				},
			)).Build()
		rtest.ValidateAndNormalize(t, registry, route1)

		// Now witness a route with multiple parents, overlapping the other route.
		requireTracking(t, m, route2, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, fooSvc, fooComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes, apiComputedRoutes)

		requireTracking(t, m, apiFail, apiComputedRoutes)
		requireTracking(t, m, wwwFail, wwwComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, barFail, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, route1, apiComputedRoutes)
		// skip re-verifying route2
		// requireTracking(t, m, route2, apiComputedRoutes, fooComputedRoutes)

		// verify other helper methods
		prototest.AssertElementsMatch(t, []*pbresource.Reference{wwwSvcRef}, m.BackendServiceRefsByRouteID(route1.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef}, m.ParentServiceRefsByRouteID(route1.Id))

		prototest.AssertElementsMatch(t, []*pbresource.Reference{barSvcRef}, m.BackendServiceRefsByRouteID(route2.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef, fooSvcRef}, m.ParentServiceRefsByRouteID(route2.Id))

		require.Empty(t, m.RouteIDsByBackendServiceRef(apiSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id, route2.Id}, m.RouteIDsByParentServiceRef(apiSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id}, m.RouteIDsByBackendServiceRef(wwwSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(wwwSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByBackendServiceRef(barSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(barSvcRef))

		require.Empty(t, m.RouteIDsByBackendServiceRef(fooSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByParentServiceRef(fooSvcRef))

		for _, ref := range []*pbresource.Reference{zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	testutil.RunStep(t, "update the failover policy to cross services", func(t *testing.T) {
		apiFail = newFailPolicy("api",
			newRef(pbcatalog.ServiceType, "foo"),
			newRef(pbcatalog.ServiceType, "zim"))
		requireTracking(t, m, apiFail, apiComputedRoutes)

		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, fooSvc, fooComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes, apiComputedRoutes)

		// skipping verification of apiFail b/c it happened above already
		// requireTracking(t, m, apiFail, apiComputedRoutes)
		requireTracking(t, m, wwwFail, wwwComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, barFail, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, route1, apiComputedRoutes)
		requireTracking(t, m, route2, apiComputedRoutes, fooComputedRoutes)

		// verify other helper methods
		prototest.AssertElementsMatch(t, []*pbresource.Reference{wwwSvcRef}, m.BackendServiceRefsByRouteID(route1.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef}, m.ParentServiceRefsByRouteID(route1.Id))

		prototest.AssertElementsMatch(t, []*pbresource.Reference{barSvcRef}, m.BackendServiceRefsByRouteID(route2.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef, fooSvcRef}, m.ParentServiceRefsByRouteID(route2.Id))

		require.Empty(t, m.RouteIDsByBackendServiceRef(apiSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id, route2.Id}, m.RouteIDsByParentServiceRef(apiSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id}, m.RouteIDsByBackendServiceRef(wwwSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(wwwSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByBackendServiceRef(barSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(barSvcRef))

		require.Empty(t, m.RouteIDsByBackendServiceRef(fooSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByParentServiceRef(fooSvcRef))

		for _, ref := range []*pbresource.Reference{zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	testutil.RunStep(t, "set a new failover policy for a service in route2", func(t *testing.T) {
		barFail = newFailPolicy("bar",
			newRef(pbcatalog.ServiceType, "gir"))
		requireTracking(t, m, barFail, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, fooSvc, fooComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes, apiComputedRoutes)

		requireTracking(t, m, apiFail, apiComputedRoutes)
		requireTracking(t, m, wwwFail, wwwComputedRoutes, apiComputedRoutes)
		// skipping verification of barFail b/c it happened above already
		// requireTracking(t, m, barFail, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, route1, apiComputedRoutes)
		requireTracking(t, m, route2, apiComputedRoutes, fooComputedRoutes)

		// verify other helper methods
		prototest.AssertElementsMatch(t, []*pbresource.Reference{wwwSvcRef}, m.BackendServiceRefsByRouteID(route1.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef}, m.ParentServiceRefsByRouteID(route1.Id))

		prototest.AssertElementsMatch(t, []*pbresource.Reference{barSvcRef}, m.BackendServiceRefsByRouteID(route2.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef, fooSvcRef}, m.ParentServiceRefsByRouteID(route2.Id))

		require.Empty(t, m.RouteIDsByBackendServiceRef(apiSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id, route2.Id}, m.RouteIDsByParentServiceRef(apiSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route1.Id}, m.RouteIDsByBackendServiceRef(wwwSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(wwwSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByBackendServiceRef(barSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(barSvcRef))

		require.Empty(t, m.RouteIDsByBackendServiceRef(fooSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByParentServiceRef(fooSvcRef))

		for _, ref := range []*pbresource.Reference{zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	testutil.RunStep(t, "delete first route", func(t *testing.T) {
		m.UntrackXRoute(route1.Id)
		route1 = nil

		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, fooSvc, fooComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes, apiComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes)

		requireTracking(t, m, apiFail, apiComputedRoutes)
		requireTracking(t, m, wwwFail, wwwComputedRoutes)
		requireTracking(t, m, barFail, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, route2, apiComputedRoutes, fooComputedRoutes)

		// verify other helper methods
		prototest.AssertElementsMatch(t, []*pbresource.Reference{barSvcRef}, m.BackendServiceRefsByRouteID(route2.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef, fooSvcRef}, m.ParentServiceRefsByRouteID(route2.Id))

		require.Empty(t, m.RouteIDsByBackendServiceRef(apiSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByParentServiceRef(apiSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByBackendServiceRef(barSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(barSvcRef))

		require.Empty(t, m.RouteIDsByBackendServiceRef(fooSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByParentServiceRef(fooSvcRef))

		for _, ref := range []*pbresource.Reference{wwwSvcRef, zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	testutil.RunStep(t, "delete all failover", func(t *testing.T) {
		m.UntrackFailoverPolicy(apiFail.Id)
		m.UntrackFailoverPolicy(wwwFail.Id)
		m.UntrackFailoverPolicy(barFail.Id)

		apiFail = nil
		wwwFail = nil
		barFail = nil

		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes, apiComputedRoutes, fooComputedRoutes)

		requireTracking(t, m, fooSvc, fooComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes)

		requireTracking(t, m, route2, apiComputedRoutes, fooComputedRoutes)

		// verify other helper methods
		prototest.AssertElementsMatch(t, []*pbresource.Reference{barSvcRef}, m.BackendServiceRefsByRouteID(route2.Id))
		prototest.AssertElementsMatch(t, []*pbresource.Reference{apiSvcRef, fooSvcRef}, m.ParentServiceRefsByRouteID(route2.Id))

		require.Empty(t, m.RouteIDsByBackendServiceRef(apiSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByParentServiceRef(apiSvcRef))

		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByBackendServiceRef(barSvcRef))
		require.Empty(t, m.RouteIDsByParentServiceRef(barSvcRef))

		require.Empty(t, m.RouteIDsByBackendServiceRef(fooSvcRef))
		prototest.AssertElementsMatch(t, []*pbresource.ID{route2.Id}, m.RouteIDsByParentServiceRef(fooSvcRef))

		for _, ref := range []*pbresource.Reference{wwwSvcRef, zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	testutil.RunStep(t, "delete second route", func(t *testing.T) {
		m.UntrackXRoute(route2.Id)
		route2 = nil

		requireTracking(t, m, apiSvc, apiComputedRoutes)
		requireTracking(t, m, wwwSvc, wwwComputedRoutes)
		requireTracking(t, m, barSvc, barComputedRoutes)

		requireTracking(t, m, fooSvc, fooComputedRoutes)
		requireTracking(t, m, zimSvc, zimComputedRoutes)
		requireTracking(t, m, girSvc, girComputedRoutes)

		requireTracking(t, m, apiDest, apiComputedRoutes)
		requireTracking(t, m, wwwDest, wwwComputedRoutes)

		// verify other helper methods
		for _, ref := range []*pbresource.Reference{apiSvcRef, wwwSvcRef, barSvcRef, fooSvcRef, zimSvcRef, girSvcRef} {
			require.Empty(t, m.RouteIDsByBackendServiceRef(ref))
			require.Empty(t, m.RouteIDsByParentServiceRef(ref))
		}
	})

	testutil.RunStep(t, "removal of a parent still triggers for old computed routes until the bound reference is cleared", func(t *testing.T) {
		route1 = rtest.Resource(typ, "route-1").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, newRoute(t,
				[]*pbmesh.ParentReference{
					{Ref: newRef(pbcatalog.ServiceType, "bar")},
					{Ref: newRef(pbcatalog.ServiceType, "foo")},
				},
				[]*pbmesh.BackendReference{
					newBackendRef("api"),
				},
			)).Build()
		rtest.ValidateAndNormalize(t, registry, route1)

		requireTracking(t, m, route1, barComputedRoutes, fooComputedRoutes)

		// Simulate a Reconcile that would update the mapper.
		//
		// NOTE: we do not ValidateAndNormalize these since the mapper doesn't use the data.
		fooCR := rtest.ResourceID(fooComputedRoutes).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, &pbmesh.ComputedRoutes{
				BoundReferences: []*pbresource.Reference{
					apiSvcRef,
					fooSvcRef,
					resource.Reference(route1.Id, ""),
				},
			}).Build()
		m.TrackComputedRoutes(rtest.MustDecode[*pbmesh.ComputedRoutes](t, fooCR))

		barCR := rtest.ResourceID(barComputedRoutes).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, &pbmesh.ComputedRoutes{
				BoundReferences: []*pbresource.Reference{
					apiSvcRef,
					barSvcRef,
					resource.Reference(route1.Id, ""),
				},
			}).Build()
		m.TrackComputedRoutes(rtest.MustDecode[*pbmesh.ComputedRoutes](t, barCR))

		// Still has the same tracking.
		requireTracking(t, m, route1, barComputedRoutes, fooComputedRoutes)

		// Now change the route to remove "bar"

		route1 = rtest.Resource(typ, "route-1").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, newRoute(t,
				[]*pbmesh.ParentReference{
					{Ref: newRef(pbcatalog.ServiceType, "foo")},
				},
				[]*pbmesh.BackendReference{
					newBackendRef("api"),
				},
			)).Build()
		rtest.ValidateAndNormalize(t, registry, route1)

		// Now we see that it still emits the event for bar, so we get a chance to update it.
		requireTracking(t, m, route1, barComputedRoutes, fooComputedRoutes)

		// Update the bound references on 'bar' to remove the route
		barCR = rtest.ResourceID(barComputedRoutes).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, &pbmesh.ComputedRoutes{
				BoundReferences: []*pbresource.Reference{
					apiSvcRef,
					barSvcRef,
				},
			}).Build()
		m.TrackComputedRoutes(rtest.MustDecode[*pbmesh.ComputedRoutes](t, barCR))

		// Now 'bar' no longer has a link to the route.
		requireTracking(t, m, route1, fooComputedRoutes)
	})
}

func requireTracking(
	t *testing.T,
	mapper *Mapper,
	res *pbresource.Resource,
	computedRoutesIDs ...*pbresource.ID,
) {
	t.Helper()

	require.NotNil(t, res)

	var (
		reqs []controller.Request
		err  error
	)
	switch {
	case resource.EqualType(pbmesh.HTTPRouteType, res.Id.Type):
		reqs, err = mapper.MapHTTPRoute(context.Background(), controller.Runtime{}, res)
	case resource.EqualType(pbmesh.GRPCRouteType, res.Id.Type):
		reqs, err = mapper.MapGRPCRoute(context.Background(), controller.Runtime{}, res)
	case resource.EqualType(pbmesh.TCPRouteType, res.Id.Type):
		reqs, err = mapper.MapTCPRoute(context.Background(), controller.Runtime{}, res)
	case resource.EqualType(pbmesh.DestinationPolicyType, res.Id.Type):
		reqs, err = mapper.MapDestinationPolicy(context.Background(), controller.Runtime{}, res)
	case resource.EqualType(pbcatalog.FailoverPolicyType, res.Id.Type):
		reqs, err = mapper.MapFailoverPolicy(context.Background(), controller.Runtime{}, res)
	case resource.EqualType(pbcatalog.ServiceType, res.Id.Type):
		reqs, err = mapper.MapService(context.Background(), controller.Runtime{}, res)
	default:
		t.Fatalf("unhandled resource type: %s", resource.TypeToString(res.Id.Type))
	}

	require.NoError(t, err)
	reqs = testDeduplicateRequests(reqs)
	require.Len(t, reqs, len(computedRoutesIDs))
	for _, computedRoutesID := range computedRoutesIDs {
		require.NotNil(t, computedRoutesID)
		prototest.AssertContainsElement(t, reqs, controller.Request{ID: computedRoutesID})
	}
}

func newBackendRef(name string) *pbmesh.BackendReference {
	return &pbmesh.BackendReference{
		Ref: newRef(pbcatalog.ServiceType, name),
	}
}

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return rtest.Resource(typ, name).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Reference("")
}

func newID(typ *pbresource.Type, name string) *pbresource.ID {
	return rtest.Resource(typ, name).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		ID()
}

func testDeduplicateRequests(reqs []controller.Request) []controller.Request {
	type resID struct {
		resource.ReferenceKey
		UID string
	}

	out := make([]controller.Request, 0, len(reqs))
	seen := make(map[resID]struct{})

	for _, req := range reqs {
		rid := resID{
			ReferenceKey: resource.NewReferenceKey(req.ID),
			UID:          req.ID.Uid,
		}
		if _, ok := seen[rid]; !ok {
			out = append(out, req)
			seen[rid] = struct{}{}
		}
	}

	return out
}
