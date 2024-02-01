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
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestComputeNewDestPolicyPortConditions(t *testing.T) {
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

	newDestPolicy := func(name string, portConfigs map[string]*pbmesh.DestinationConfig) *types.DecodedDestinationPolicy {
		policy := rtest.Resource(pbmesh.DestinationPolicyType, name).
			WithData(t, &pbmesh.DestinationPolicy{PortConfigs: portConfigs}).
			Build()
		rtest.ValidateAndNormalize(t, registry, policy)

		dec, err := resource.Decode[*pbmesh.DestinationPolicy](policy)
		require.NoError(t, err)
		return dec
	}

	t.Run("with no service", func(t *testing.T) {
		sg := newTestServiceGetter()
		got := computeNewDestPolicyPortConditions(sg, resource.NewReferenceKey(
			newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy())),
			newDestPolicy("dest", map[string]*pbmesh.DestinationConfig{
				"http": defaultDestConfig(),
			}))
		require.Len(t, got, 1)
		prototest.AssertContainsElement(t, got, ConditionDestinationServiceNotFound(
			newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
		))
	})

	t.Run("with service and using missing port", func(t *testing.T) {
		sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
			"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
			"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
		}))
		got := computeNewDestPolicyPortConditions(sg, resource.NewReferenceKey(
			newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy())),
			newDestPolicy("dest", map[string]*pbmesh.DestinationConfig{
				"grpc": defaultDestConfig(),
			}))
		require.Len(t, got, 1)
		prototest.AssertContainsElement(t, got, ConditionUnknownDestinationPort(
			newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
			"grpc",
		))
	})

	t.Run("with service and using duplicate port", func(t *testing.T) {
		sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
			"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
			"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
		}))
		got := computeNewDestPolicyPortConditions(sg, resource.NewReferenceKey(
			newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy())),
			newDestPolicy("dest", map[string]*pbmesh.DestinationConfig{
				"http": defaultDestConfig(),
				"8080": defaultDestConfig(),
			}))
		require.Len(t, got, 1)
		prototest.AssertContainsElement(t, got, ConditionConflictDestinationPort(
			newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy()),
			&pbcatalog.ServicePort{VirtualPort: 8080, TargetPort: "http"},
		))
	})

	t.Run("with service and using correct port", func(t *testing.T) {
		sg := newTestServiceGetter(newService("api", map[string]protocolAndVirtualPort{
			"http": {pbcatalog.Protocol_PROTOCOL_HTTP, 8080},
			"mesh": {pbcatalog.Protocol_PROTOCOL_MESH, 20000},
		}))
		got := computeNewDestPolicyPortConditions(sg, resource.NewReferenceKey(
			newRef(pbcatalog.ServiceType, "api", resource.DefaultNamespacedTenancy())),
			newDestPolicy("dest", map[string]*pbmesh.DestinationConfig{
				"http": defaultDestConfig(),
			}))
		require.Empty(t, got)
	})
}
