// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestUnified_AllMappingsToProxyStateTemplate(t *testing.T) {
	var (
		destCache = sidecarproxycache.NewDestinationsCache()
		// proxyCfgCache = sidecarproxycache.NewProxyConfigurationCache()
		routesCache = sidecarproxycache.NewComputedRoutesCache()
		mapper      = New(destCache, nil, routesCache, nil)

		client = svctest.RunResourceService(t, types.Register, catalog.RegisterTypes)
	)

	anyServiceData := &pbcatalog.Service{
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

	anyWorkloadPorts := map[string]*pbcatalog.WorkloadPort{
		"tcp1": {Port: 8080},
		"tcp2": {Port: 8081},
		"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
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
		destWorkload1 = newID(pbcatalog.WorkloadType, "dest-workload-1")
		destWorkload2 = newID(pbcatalog.WorkloadType, "dest-workload-2")

		destProxy1 = resource.ReplaceType(pbmesh.ProxyStateTemplateType, destWorkload1)
		destProxy2 = resource.ReplaceType(pbmesh.ProxyStateTemplateType, destWorkload2)
	)

	// Endpoints for original destination
	destEndpoints := resourcetest.ResourceID(resource.ReplaceType(pbcatalog.ServiceEndpointsType, destService.Id)).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: destWorkload1,
					Ports:     anyWorkloadPorts,
				},
				{
					TargetRef: destWorkload2,
					Ports:     anyWorkloadPorts,
				},
			},
		}).
		Build()

	var (
		targetWorkload1       = newID(pbcatalog.WorkloadType, "target-workload-1")
		targetWorkload2       = newID(pbcatalog.WorkloadType, "target-workload-2")
		targetWorkload3       = newID(pbcatalog.WorkloadType, "target-workload-3")
		backupTargetWorkload1 = newID(pbcatalog.WorkloadType, "backup-target-workload-1")
		backupTargetWorkload2 = newID(pbcatalog.WorkloadType, "backup-target-workload-2")
		backupTargetWorkload3 = newID(pbcatalog.WorkloadType, "backup-target-workload-3")

		targetProxy1       = resource.ReplaceType(pbmesh.ProxyStateTemplateType, targetWorkload1)
		targetProxy2       = resource.ReplaceType(pbmesh.ProxyStateTemplateType, targetWorkload2)
		targetProxy3       = resource.ReplaceType(pbmesh.ProxyStateTemplateType, targetWorkload3)
		backupTargetProxy1 = resource.ReplaceType(pbmesh.ProxyStateTemplateType, backupTargetWorkload1)
		backupTargetProxy2 = resource.ReplaceType(pbmesh.ProxyStateTemplateType, backupTargetWorkload2)
		backupTargetProxy3 = resource.ReplaceType(pbmesh.ProxyStateTemplateType, backupTargetWorkload3)
	)

	// Endpoints for actual destination
	targetEndpoints := resourcetest.ResourceID(resource.ReplaceType(pbcatalog.ServiceEndpointsType, targetService.Id)).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: targetWorkload1,
					Ports:     anyWorkloadPorts,
				},
				{
					TargetRef: targetWorkload2,
					Ports:     anyWorkloadPorts,
				},
				{
					TargetRef: targetWorkload3,
					Ports:     anyWorkloadPorts,
				},
			},
		}).
		Build()
	backupTargetEndpoints := resourcetest.ResourceID(resource.ReplaceType(pbcatalog.ServiceEndpointsType, backupTargetService.Id)).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: backupTargetWorkload1,
					Ports:     anyWorkloadPorts,
				},
				{
					TargetRef: backupTargetWorkload2,
					Ports:     anyWorkloadPorts,
				},
				{
					TargetRef: backupTargetWorkload3,
					Ports:     anyWorkloadPorts,
				},
			},
		}).
		Build()

	var (
		sourceProxy1 = newID(pbmesh.ProxyStateTemplateType, "src-workload-1")
		sourceProxy2 = newID(pbmesh.ProxyStateTemplateType, "src-workload-2")
		sourceProxy3 = newID(pbmesh.ProxyStateTemplateType, "src-workload-3")
		sourceProxy4 = newID(pbmesh.ProxyStateTemplateType, "src-workload-4")
		sourceProxy5 = newID(pbmesh.ProxyStateTemplateType, "src-workload-5")
		sourceProxy6 = newID(pbmesh.ProxyStateTemplateType, "src-workload-6")
	)

	destination1 := intermediate.CombinedDestinationRef{
		ServiceRef: destServiceRef,
		Port:       "tcp1",
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(sourceProxy1): {},
			resource.NewReferenceKey(sourceProxy2): {},
		},
	}
	destination2 := intermediate.CombinedDestinationRef{
		ServiceRef: destServiceRef,
		Port:       "tcp2",
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(sourceProxy3): {},
			resource.NewReferenceKey(sourceProxy4): {},
		},
	}
	destination3 := intermediate.CombinedDestinationRef{
		ServiceRef: destServiceRef,
		Port:       "mesh",
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(sourceProxy5): {},
			resource.NewReferenceKey(sourceProxy6): {},
		},
	}

	routesCache.TrackComputedRoutes(webRoutes)
	destCache.WriteDestination(destination1)
	destCache.WriteDestination(destination2)
	destCache.WriteDestination(destination3)

	t.Run("ServiceEndpoints", func(t *testing.T) {
		t.Run("map dest endpoints", func(t *testing.T) {
			requests, err := mapper.MapServiceEndpointsToProxyStateTemplate(
				context.Background(),
				controller.Runtime{Client: client},
				destEndpoints,
			)
			require.NoError(t, err)

			expRequests := []controller.Request{
				// Just wakeup proxies for these workloads.
				{ID: destProxy1},
				{ID: destProxy2},
			}

			prototest.AssertElementsMatch(t, expRequests, requests)
		})

		t.Run("map target endpoints (TCPRoute)", func(t *testing.T) {
			requests, err := mapper.MapServiceEndpointsToProxyStateTemplate(
				context.Background(),
				controller.Runtime{Client: client},
				targetEndpoints,
			)
			require.NoError(t, err)

			requests = testDeduplicateRequests(requests)

			expRequests := []controller.Request{
				// Wakeup proxies for these workloads.
				{ID: targetProxy1},
				{ID: targetProxy2},
				{ID: targetProxy3},
				// Also wakeup things that have destService as a destination b/c of the TCPRoute reference.
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
			requests, err := mapper.MapServiceEndpointsToProxyStateTemplate(
				context.Background(),
				controller.Runtime{Client: client},
				backupTargetEndpoints,
			)
			require.NoError(t, err)

			requests = testDeduplicateRequests(requests)

			expRequests := []controller.Request{
				// Wakeup proxies for these workloads.
				{ID: backupTargetProxy1},
				{ID: backupTargetProxy2},
				{ID: backupTargetProxy3},
				// Also wakeup things that have destService as a destination b/c of the FailoverPolicy reference.
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

	t.Run("Service", func(t *testing.T) {
		t.Run("map dest service", func(t *testing.T) {
			requests, err := mapper.MapServiceToProxyStateTemplate(
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
		})

		t.Run("map target endpoints (TCPRoute)", func(t *testing.T) {
			requests, err := mapper.MapServiceToProxyStateTemplate(
				context.Background(),
				controller.Runtime{Client: client},
				targetService,
			)
			require.NoError(t, err)

			requests = testDeduplicateRequests(requests)

			// No upstream referrs to target directly.
			expRequests := []controller.Request{}

			prototest.AssertElementsMatch(t, expRequests, requests)
		})
	})

	t.Run("ComputedRoutes", func(t *testing.T) {
		t.Run("map web routes", func(t *testing.T) {
			requests, err := mapper.MapComputedRoutesToProxyStateTemplate(
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

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return resource.Reference(newID(typ, name), "")
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
