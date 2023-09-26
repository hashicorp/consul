// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMapDestinationsToProxyStateTemplate(t *testing.T) {
	client := svctest.RunResourceService(t, types.Register, catalog.RegisterTypes)
	webWorkload1 := resourcetest.Resource(pbcatalog.WorkloadType, "web-abc").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.1"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
		}).
		Write(t, client)
	webWorkload2 := resourcetest.Resource(pbcatalog.WorkloadType, "web-def").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.2"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
		}).
		Write(t, client)
	webWorkload3 := resourcetest.Resource(pbcatalog.WorkloadType, "non-prefix-web").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.3"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
		}).
		Write(t, client)

	var (
		api1ServiceRef = resourcetest.Resource(pbcatalog.ServiceType, "api-1").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				ReferenceNoSection()
		api2ServiceRef = resourcetest.Resource(pbcatalog.ServiceType, "api-2").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				ReferenceNoSection()
	)

	webDestinationsData := &pbmesh.Destinations{
		Workloads: &pbcatalog.WorkloadSelector{
			Names:    []string{"non-prefix-web"},
			Prefixes: []string{"web"},
		},
		Destinations: []*pbmesh.Destination{
			{
				DestinationRef:  api1ServiceRef,
				DestinationPort: "tcp",
			},
			{
				DestinationRef:  api2ServiceRef,
				DestinationPort: "tcp1",
			},
			{
				DestinationRef:  api2ServiceRef,
				DestinationPort: "tcp2",
			},
		},
	}

	webDestinations := resourcetest.Resource(pbmesh.DestinationsType, "web-destinations").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, webDestinationsData).
		Write(t, client)

	c := sidecarproxycache.NewDestinationsCache()
	mapper := &Mapper{destinationsCache: c}

	expRequests := []controller.Request{
		{ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, webWorkload1.Id)},
		{ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, webWorkload2.Id)},
		{ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, webWorkload3.Id)},
	}

	requests, err := mapper.MapDestinationsToProxyStateTemplate(context.Background(), controller.Runtime{Client: client}, webDestinations)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, expRequests, requests)

	var (
		proxy1ID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, webWorkload1.Id.Name).
				WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		proxy2ID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, webWorkload2.Id.Name).
				WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		proxy3ID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, webWorkload3.Id.Name).
				WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	)
	for _, u := range webDestinationsData.Destinations {
		expDestination := intermediate.CombinedDestinationRef{
			ServiceRef:             u.DestinationRef,
			Port:                   u.DestinationPort,
			ExplicitDestinationsID: webDestinations.Id,
			SourceProxies: map[resource.ReferenceKey]struct{}{
				resource.NewReferenceKey(proxy1ID): {},
				resource.NewReferenceKey(proxy2ID): {},
				resource.NewReferenceKey(proxy3ID): {},
			},
		}
		actualDestination, found := c.ReadDestination(u.DestinationRef, u.DestinationPort)
		require.True(t, found)
		prototest.AssertDeepEqual(t, expDestination, actualDestination)
	}
}
