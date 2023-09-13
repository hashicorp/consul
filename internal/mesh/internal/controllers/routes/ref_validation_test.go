// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"testing"

	"github.com/stretchr/testify/require"

	catalogapi "github.com/hashicorp/consul/api/catalog/v2beta1"
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

	newService := func(name string, ports map[string]pbcatalog.Protocol) *types.DecodedService {
		var portSlice []*pbcatalog.ServicePort
		for name, proto := range ports {
			portSlice = append(portSlice, &pbcatalog.ServicePort{
				TargetPort: name,
				Protocol:   proto,
			})
		}
		svc := rtest.Resource(catalogapi.ServiceType, name).
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
				newParentRef(newRef(catalogapi.ServiceType, "api"), ""),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionMissingParentRef(
				newRef(catalogapi.ServiceType, "api"),
			))
		})

		t.Run("with service but no mesh port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(catalogapi.ServiceType, "api"), ""),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionParentRefOutsideMesh(
				newRef(catalogapi.ServiceType, "api"),
			))
		})

		t.Run("with service but using mesh port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
				"mesh": pbcatalog.Protocol_PROTOCOL_MESH,
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(catalogapi.ServiceType, "api"), "mesh"),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionParentRefUsingMeshPort(
				newRef(catalogapi.ServiceType, "api"),
				"mesh",
			))
		})

		t.Run("with service and using missing port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
				"mesh": pbcatalog.Protocol_PROTOCOL_MESH,
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(catalogapi.ServiceType, "api"), "web"),
			}, nil)
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionUnknownParentRefPort(
				newRef(catalogapi.ServiceType, "api"),
				"web",
			))
		})

		t.Run("with service and using empty port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
				"mesh": pbcatalog.Protocol_PROTOCOL_MESH,
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(catalogapi.ServiceType, "api"), ""),
			}, nil)
			require.Empty(t, got)
		})

		t.Run("with service and using correct port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
				"mesh": pbcatalog.Protocol_PROTOCOL_MESH,
			}))
			got := computeNewRouteRefConditions(sg, []*pbmesh.ParentReference{
				newParentRef(newRef(catalogapi.ServiceType, "api"), "http"),
			}, nil)
			require.Empty(t, got)
		})
	})

	t.Run("backend refs", func(t *testing.T) {
		t.Run("with no service", func(t *testing.T) {
			sg := newTestServiceGetter()
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(catalogapi.ServiceType, "api"), "", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionMissingBackendRef(
				newRef(catalogapi.ServiceType, "api"),
			))
		})

		t.Run("with service but no mesh port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(catalogapi.ServiceType, "api"), "", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionBackendRefOutsideMesh(
				newRef(catalogapi.ServiceType, "api"),
			))
		})

		t.Run("with service but using mesh port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
				"mesh": pbcatalog.Protocol_PROTOCOL_MESH,
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(catalogapi.ServiceType, "api"), "mesh", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionBackendRefUsingMeshPort(
				newRef(catalogapi.ServiceType, "api"),
				"mesh",
			))
		})

		t.Run("with service and using missing port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
				"mesh": pbcatalog.Protocol_PROTOCOL_MESH,
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(catalogapi.ServiceType, "api"), "web", ""),
			})
			require.Len(t, got, 1)
			prototest.AssertContainsElement(t, got, ConditionUnknownBackendRefPort(
				newRef(catalogapi.ServiceType, "api"),
				"web",
			))
		})

		t.Run("with service and using empty port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
				"mesh": pbcatalog.Protocol_PROTOCOL_MESH,
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(catalogapi.ServiceType, "api"), "", ""),
			})
			require.Empty(t, got)
		})

		t.Run("with service and using correct port", func(t *testing.T) {
			sg := newTestServiceGetter(newService("api", map[string]pbcatalog.Protocol{
				"http": pbcatalog.Protocol_PROTOCOL_HTTP,
				"mesh": pbcatalog.Protocol_PROTOCOL_MESH,
			}))
			got := computeNewRouteRefConditions(sg, nil, []*pbmesh.BackendReference{
				newBackendRef(newRef(catalogapi.ServiceType, "api"), "http", ""),
			})
			require.Empty(t, got)
		})
	})
}

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return rtest.Resource(typ, name).
		WithTenancy(resource.DefaultNamespacedTenancy()).
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
