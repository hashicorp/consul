package sidecarproxymapper

import (
	"context"
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestMapDestinationsToProxyStateTemplate(t *testing.T) {
	client := svctest.RunResourceService(t, types.Register, catalog.RegisterTypes)
	webWorkload1 := resourcetest.Resource(catalog.WorkloadType, "web-abc").
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.1"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
		}).
		Write(t, client)
	webWorkload2 := resourcetest.Resource(catalog.WorkloadType, "web-def").
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.2"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
		}).
		Write(t, client)
	webWorkload3 := resourcetest.Resource(catalog.WorkloadType, "non-prefix-web").
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.3"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
		}).
		Write(t, client)

	webDestinationsData := &pbmesh.Upstreams{
		Workloads: &pbcatalog.WorkloadSelector{
			Names:    []string{"non-prefix-web"},
			Prefixes: []string{"web"},
		},
		Upstreams: []*pbmesh.Upstream{
			{
				DestinationRef:  resourcetest.Resource(catalog.ServiceType, "api-1").ReferenceNoSection(),
				DestinationPort: "tcp",
			},
			{
				DestinationRef:  resourcetest.Resource(catalog.ServiceType, "api-2").ReferenceNoSection(),
				DestinationPort: "tcp1",
			},
			{
				DestinationRef:  resourcetest.Resource(catalog.ServiceType, "api-2").ReferenceNoSection(),
				DestinationPort: "tcp2",
			},
		},
	}

	webDestinations := resourcetest.Resource(types.UpstreamsType, "web-destinations").
		WithData(t, webDestinationsData).
		Write(t, client)

	c := sidecarproxycache.NewDestinationsCache()
	mapper := &Mapper{destinationsCache: c}

	expRequests := []controller.Request{
		{ID: resource.ReplaceType(types.ProxyStateTemplateType, webWorkload1.Id)},
		{ID: resource.ReplaceType(types.ProxyStateTemplateType, webWorkload2.Id)},
		{ID: resource.ReplaceType(types.ProxyStateTemplateType, webWorkload3.Id)},
	}

	requests, err := mapper.MapDestinationsToProxyStateTemplate(context.Background(), controller.Runtime{Client: client}, webDestinations)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, expRequests, requests)

	//var expDestinations []*intermediate.CombinedDestinationRef
	proxy1ID := resourcetest.Resource(types.ProxyStateTemplateType, webWorkload1.Id.Name).ID()
	proxy2ID := resourcetest.Resource(types.ProxyStateTemplateType, webWorkload2.Id.Name).ID()
	proxy3ID := resourcetest.Resource(types.ProxyStateTemplateType, webWorkload3.Id.Name).ID()
	for _, u := range webDestinationsData.Upstreams {
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
