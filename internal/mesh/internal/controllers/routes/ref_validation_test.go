// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"testing"

	"github.com/stretchr/testify/require"

	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestComputeNewRouteRefConditions(t *testing.T) {
	registry := resource.NewRegistry()
	types.Register(registry)
	catalog.RegisterTypes(registry)

	type protocolAndVirtualPort struct {
		protocol    pbcatalog.Protocol
		virtualPort uint32
	}

	newService := func(name string, ports map[string]protocolAndVirtualPort) *types.DecodedService {
		var portSlice []*pbcatalog.ServicePort
		for targetPort, pv := range ports {
			portSlice = append(portSlice, &pbcatalog.ServicePort{
				TargetPort:  targetPort,
				VirtualPort: pv.virtualPort,
				Protocol:    pv.protocol,
			})
		}
		svc := rtest.Resource(pbcatalog.ServiceType, name).
			WithData(t, &pbcatalog.Service{Ports: portSlice}).
			Build()
		rtest.ValidateAndNormalize(t, registry, svc)

		dec, err := resource.Decode[*pbcatalog.Service](svc)
		require.NoError(t, err)
		return dec
	}

	t.Run("no refs", func(t *testing.T) {
		sg := newTestServiceGetter()
		got := computeNewRouteRefConditions(sg, nil, nil)
		require.Empty(t, got)
	})

	t.Run("parent refs", func(t *testing.T) {
		t.Run("with no service", func(t *testing.T) {
			sg := newTestServiceGetter()
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), ""),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionMissingParentRef(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
			))
		})

		t.Run("with service but no mesh port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), ""),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionParentRefOutsideMesh(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
			))
		})

		t.Run("with service but using mesh port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "mesh"),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionParentRefUsingMeshPort(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
				"mesh",
			))
		})

		t.Run("with service and using missing port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "web"),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionUnknownParentRefPort(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
				"web",
			))
		})

		t.Run("with service and using duplicate port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "http"),
				newParentRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "8080"),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionConflictParentRefPort(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
				"http",
			))
		})

		t.Run("with service and using empty port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), ""),
			}, nil)
			require.Empty(t, got)
		})

		t.Run("with service and using correct port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "http"),
			}, nil)
			require.Empty(t, got)
		})
	})

	t.Run("backend refs", func(t *testing.T) {
		t.Run("with no service", func(t *testing.T) {
			sg := newTestServiceGetter()
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionMissingBackendRef(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
			))
		})

		t.Run("with service but no mesh port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionBackendRefOutsideMesh(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
			))
		})

		t.Run("with service but using mesh port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "mesh", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionBackendRefUsingMeshPort(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
				"mesh",
			))
		})

		t.Run("with service and using missing port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "web", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionUnknownBackendRefPort(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
				"web",
			))
		})

		t.Run("with service and using duplicate port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "http", ""),
				newBackendRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "8080", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionConflictBackendRefPort(
				newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
				"http",
			))
		})

		t.Run("with service and using empty port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "", ""),
			})
			require.Empty(t, got)
		})

		t.Run("with service and using correct port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
				"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
				"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()), "http", ""),
			})
			require.Empty(t, got)
		})
	})
}

func newRef(typ *pbresource.Type, name string, tenancy *pbresource.Tenancy) *pbresource.Reference {
	if tenancy == nil {
		tenancy = resource.DefaultNamespacedTenancy()
	}

	return rtest.Resource(typ, name).
		WithTenancy(tenancy).
		Reference("")
}

type testServiceGetter struct {
	services map[resource.ReferenceKey]*types.DecodedService
}

func newTestServiceGetter(svcs ...*types.DecodedService) serviceGetter {
	g := &testServiceGetter{
		services: make(map[resource.ReferenceKey]*types.DecodedService),
	}
	for _, svc := range svcs {
		g.services[resource.NewReferenceKey(svc.Resource.Id)] = svc
	}
	return g
}

func (g *testServiceGetter) GetService(ref resource.ReferenceOrID) *types.DecodedService {
	if g.services == nil {
		return nil
	}
	return g.services[resource.NewReferenceKey(ref)]
}
