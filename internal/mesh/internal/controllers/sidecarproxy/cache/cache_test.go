// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestIdentities(t *testing.T) {
	cache := New()

	identityID1 := resourcetest.Resource(pbauth.WorkloadIdentityType, "workload-identity-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	identityID2 := resourcetest.Resource(pbauth.WorkloadIdentityType, "workload-identity-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	w1 := resourcetest.Resource(pbcatalog.WorkloadType, "service-workload-1").
		WithData(t, &pbcatalog.Workload{
			Identity: identityID1.Name,
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	decW1 := resourcetest.MustDecode[*pbcatalog.Workload](t, w1)
	w2 := resourcetest.Resource(pbcatalog.WorkloadType, "service-workload-2").
		WithData(t, &pbcatalog.Workload{
			Identity: identityID2.Name,
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	decW2 := resourcetest.MustDecode[*pbcatalog.Workload](t, w2)

	// Empty cache
	require.Nil(t, cache.WorkloadsByWorkloadIdentity(identityID1))
	require.Nil(t, cache.WorkloadsByWorkloadIdentity(identityID2))

	// Insert value and fetch it.
	cache.TrackWorkload(decW1)
	require.Equal(t, []*pbresource.ID{w1.Id}, cache.WorkloadsByWorkloadIdentity(identityID1))
	require.Nil(t, cache.WorkloadsByWorkloadIdentity(identityID2))

	// Insert another value referencing the same identity.
	decW2.GetData().Identity = identityID1.Name
	cache.TrackWorkload(decW2)
	require.ElementsMatch(t, []*pbresource.ID{w1.Id, w2.Id}, cache.WorkloadsByWorkloadIdentity(identityID1))
	require.Nil(t, cache.WorkloadsByWorkloadIdentity(identityID2))

	// Now workload 1 uses identity 2
	decW1.GetData().Identity = identityID2.Name
	cache.TrackWorkload(decW1)
	require.Equal(t, []*pbresource.ID{w1.Id}, cache.WorkloadsByWorkloadIdentity(identityID2))
	require.Equal(t, []*pbresource.ID{w2.Id}, cache.WorkloadsByWorkloadIdentity(identityID1))

	// Untrack workload 2
	cache.UntrackWorkload(w2.Id)
	require.Equal(t, []*pbresource.ID{w1.Id}, cache.WorkloadsByWorkloadIdentity(identityID2))
	require.Nil(t, cache.WorkloadsByWorkloadIdentity(identityID1))

	// Untrack workload 1
	cache.UntrackWorkload(w1.Id)
	require.Nil(t, cache.WorkloadsByWorkloadIdentity(identityID2))
	require.Nil(t, cache.WorkloadsByWorkloadIdentity(identityID1))
}

func TestMapComputedTrafficPermissions(t *testing.T) {
	client := svctest.RunResourceService(t, types.Register, catalog.RegisterTypes)
	ctp := resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, "workload-identity-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbauth.ComputedTrafficPermissions{}).
		Build()

	c := New()

	// Empty results when the cache isn't populated.
	requests, err := c.MapComputedTrafficPermissions(context.Background(), controller.Runtime{Client: client}, ctp)
	require.NoError(t, err)
	require.Len(t, requests, 0)

	identityID1 := resourcetest.Resource(pbauth.WorkloadIdentityType, "workload-identity-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	w1 := resourcetest.Resource(pbcatalog.WorkloadType, "service-workload-1").
		WithData(t, &pbcatalog.Workload{
			Identity: identityID1.Name,
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	decW1 := resourcetest.MustDecode[*pbcatalog.Workload](t, w1)
	w2 := resourcetest.Resource(pbcatalog.WorkloadType, "service-workload-2").
		WithData(t, &pbcatalog.Workload{
			Identity: identityID1.Name,
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	decW2 := resourcetest.MustDecode[*pbcatalog.Workload](t, w2)

	c.TrackWorkload(decW1)

	// Empty results when the cache isn't populated.
	requests, err = c.MapComputedTrafficPermissions(context.Background(), controller.Runtime{Client: client}, ctp)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t,
		[]controller.Request{{ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, w1.Id)}}, requests)

	c.TrackWorkload(decW2)

	// Empty results when the cache isn't populated.
	requests, err = c.MapComputedTrafficPermissions(context.Background(), controller.Runtime{Client: client}, ctp)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, []controller.Request{
		{ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, w1.Id)},
		{ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, w2.Id)},
	}, requests)
}

func TestUnified_AllMappingsToProxyStateTemplate(t *testing.T) {
	var (
		cache  = New()
		client = svctest.RunResourceService(t, types.Register, catalog.RegisterTypes)
	)

	anyServiceData := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"src-workload"},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "tcp1",
				Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort: "tcp2",
				Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort: "mesh",
				Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
			},
		},
	}

	// The thing we link through Destinations.
	destService := resourcetest.Resource(pbcatalog.ServiceType, "web").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, anyServiceData).
		Build()
	destServiceRef := resource.Reference(destService.Id, "")

	// The thing we reach through the mesh config.
	targetService := resourcetest.Resource(pbcatalog.ServiceType, "db").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, anyServiceData).
		Build()
	targetServiceRef := resource.Reference(targetService.Id, "")

	backupTargetService := resourcetest.Resource(pbcatalog.ServiceType, "db-backup").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, anyServiceData).
		Build()
	backupTargetServiceRef := resource.Reference(backupTargetService.Id, "")

	// The way we make 'web' actually route traffic to 'db'.
	tcpRoute := resourcetest.Resource(pbmesh.TCPRouteType, "tcp-route").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbmesh.TCPRoute{
			ParentRefs: []*pbmesh.ParentReference{{
				Ref: destServiceRef,
			}},
			Rules: []*pbmesh.TCPRouteRule{{
				BackendRefs: []*pbmesh.TCPBackendRef{{
					BackendRef: &pbmesh.BackendReference{
						Ref: targetServiceRef,
					},
				}},
			}},
		}).
		Build()
	failoverPolicy := resourcetest.ResourceID(resource.ReplaceType(pbcatalog.FailoverPolicyType, targetService.Id)).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.FailoverPolicy{
			Config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{{
					Ref: backupTargetServiceRef,
				}},
			},
		}).
		Build()
	webRoutes := routestest.BuildComputedRoutes(t, resource.ReplaceType(pbmesh.ComputedRoutesType, destService.Id),
		resourcetest.MustDecode[*pbcatalog.Service](t, destService),
		resourcetest.MustDecode[*pbcatalog.Service](t, targetService),
		resourcetest.MustDecode[*pbcatalog.Service](t, backupTargetService),
		resourcetest.MustDecode[*pbmesh.TCPRoute](t, tcpRoute),
		resourcetest.MustDecode[*pbcatalog.FailoverPolicy](t, failoverPolicy),
	)

	var (
		sourceProxy1 = newID(pbmesh.ProxyStateTemplateType, "src-workload-1")
		sourceProxy2 = newID(pbmesh.ProxyStateTemplateType, "src-workload-2")
		sourceProxy3 = newID(pbmesh.ProxyStateTemplateType, "src-workload-3")
		sourceProxy4 = newID(pbmesh.ProxyStateTemplateType, "src-workload-4")
		sourceProxy5 = newID(pbmesh.ProxyStateTemplateType, "src-workload-5")
		sourceProxy6 = newID(pbmesh.ProxyStateTemplateType, "src-workload-6")
	)

	compDestProxy1 := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, sourceProxy1.Name).
		WithData(t, &pbmesh.ComputedExplicitDestinations{
			Destinations: []*pbmesh.Destination{
				{
					DestinationRef:  destServiceRef,
					DestinationPort: "tcp1",
				},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	compDestProxy2 := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, sourceProxy2.Name).
		WithData(t, &pbmesh.ComputedExplicitDestinations{
			Destinations: []*pbmesh.Destination{
				{
					DestinationRef:  destServiceRef,
					DestinationPort: "tcp1",
				},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	compDestProxy3 := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, sourceProxy3.Name).
		WithData(t, &pbmesh.ComputedExplicitDestinations{
			Destinations: []*pbmesh.Destination{
				{
					DestinationRef:  destServiceRef,
					DestinationPort: "tcp2",
				},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	compDestProxy4 := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, sourceProxy4.Name).
		WithData(t, &pbmesh.ComputedExplicitDestinations{
			Destinations: []*pbmesh.Destination{
				{
					DestinationRef:  destServiceRef,
					DestinationPort: "tcp2",
				},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	compDestProxy5 := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, sourceProxy5.Name).
		WithData(t, &pbmesh.ComputedExplicitDestinations{
			Destinations: []*pbmesh.Destination{
				{
					DestinationRef:  destServiceRef,
					DestinationPort: "mesh",
				},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	compDestProxy6 := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, sourceProxy6.Name).
		WithData(t, &pbmesh.ComputedExplicitDestinations{
			Destinations: []*pbmesh.Destination{
				{
					DestinationRef:  destServiceRef,
					DestinationPort: "mesh",
				},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	cache.trackComputedRoutes(webRoutes)
	cache.TrackComputedDestinations(resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](t, compDestProxy1))
	cache.TrackComputedDestinations(resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](t, compDestProxy2))
	cache.TrackComputedDestinations(resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](t, compDestProxy3))
	cache.TrackComputedDestinations(resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](t, compDestProxy4))
	cache.TrackComputedDestinations(resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](t, compDestProxy5))
	cache.TrackComputedDestinations(resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](t, compDestProxy6))

	t.Run("Service", func(t *testing.T) {
		t.Run("map dest service", func(t *testing.T) {
			requests, err := cache.MapService(
				context.Background(),
				controller.Runtime{Client: client},
				destService,
			)
			require.NoError(t, err)

			// Only wake up things with dest as an upstream.
			expRequests := []controller.Request{
				{ID: sourceProxy1},
				{ID: sourceProxy2},
				{ID: sourceProxy3},
				{ID: sourceProxy4},
				{ID: sourceProxy5},
				{ID: sourceProxy6},
			}

			prototest.AssertElementsMatch(t, expRequests, requests)

			// Check that service's workload selector is tracked.
			prototest.AssertElementsMatch(t,
				[]*pbresource.ID{destService.Id},
				cache.serviceSelectorTracker.GetIDsForWorkload(resource.ReplaceType(pbcatalog.WorkloadType, sourceProxy1)))
			prototest.AssertElementsMatch(t,
				[]*pbresource.ID{destService.Id},
				cache.serviceSelectorTracker.GetIDsForWorkload(resource.ReplaceType(pbcatalog.WorkloadType, sourceProxy2)))
			prototest.AssertElementsMatch(t,
				[]*pbresource.ID{destService.Id},
				cache.serviceSelectorTracker.GetIDsForWorkload(resource.ReplaceType(pbcatalog.WorkloadType, sourceProxy3)))
			prototest.AssertElementsMatch(t,
				[]*pbresource.ID{destService.Id},
				cache.serviceSelectorTracker.GetIDsForWorkload(resource.ReplaceType(pbcatalog.WorkloadType, sourceProxy4)))
			prototest.AssertElementsMatch(t,
				[]*pbresource.ID{destService.Id},
				cache.serviceSelectorTracker.GetIDsForWorkload(resource.ReplaceType(pbcatalog.WorkloadType, sourceProxy5)))
			prototest.AssertElementsMatch(t,
				[]*pbresource.ID{destService.Id},
				cache.serviceSelectorTracker.GetIDsForWorkload(resource.ReplaceType(pbcatalog.WorkloadType, sourceProxy6)))
		})

		t.Run("map target endpoints (TCPRoute)", func(t *testing.T) {
			requests, err := cache.MapService(
				context.Background(),
				controller.Runtime{Client: client},
				targetService,
			)
			require.NoError(t, err)

			requests = testDeduplicateRequests(requests)

			expRequests := []controller.Request{
				// Wakeup things that have destService as a destination b/c of the TCPRoute reference.
				{ID: sourceProxy1},
				{ID: sourceProxy2},
				{ID: sourceProxy3},
				{ID: sourceProxy4},
				{ID: sourceProxy5},
				{ID: sourceProxy6},
			}

			prototest.AssertElementsMatch(t, expRequests, requests)
		})

		t.Run("map backup target endpoints (FailoverPolicy)", func(t *testing.T) {
			requests, err := cache.MapService(
				context.Background(),
				controller.Runtime{Client: client},
				backupTargetService,
			)
			require.NoError(t, err)

			requests = testDeduplicateRequests(requests)

			expRequests := []controller.Request{
				// Wakeup things that have destService as a destination b/c of the FailoverPolicy reference.
				{ID: sourceProxy1},
				{ID: sourceProxy2},
				{ID: sourceProxy3},
				{ID: sourceProxy4},
				{ID: sourceProxy5},
				{ID: sourceProxy6},
			}

			prototest.AssertElementsMatch(t, expRequests, requests)
		})
	})

	t.Run("ComputedRoutes", func(t *testing.T) {
		t.Run("map web routes", func(t *testing.T) {
			requests, err := cache.MapComputedRoutes(
				context.Background(),
				controller.Runtime{Client: client},
				webRoutes.Resource,
			)
			require.NoError(t, err)

			// Only wake up things with dest as an upstream.
			expRequests := []controller.Request{
				{ID: sourceProxy1},
				{ID: sourceProxy2},
				{ID: sourceProxy3},
				{ID: sourceProxy4},
				{ID: sourceProxy5},
				{ID: sourceProxy6},
			}

			prototest.AssertElementsMatch(t, expRequests, requests)
		})
	})
}

func newID(typ *pbresource.Type, name string) *pbresource.ID {
	return &pbresource.ID{
		Type:    typ,
		Tenancy: resource.DefaultNamespacedTenancy(),
		Name:    name,
	}
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
