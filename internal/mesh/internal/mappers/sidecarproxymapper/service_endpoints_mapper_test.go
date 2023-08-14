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
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestMapServiceEndpointsToProxyStateTemplate(t *testing.T) {
	client := svctest.RunResourceService(t, types.Register, catalog.RegisterTypes)

	workload1 := resourcetest.Resource(catalog.WorkloadType, "workload-1").Build()
	workload2 := resourcetest.Resource(catalog.WorkloadType, "workload-2").Build()
	serviceEndpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "service").
		WithData(t, &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: workload1.Id,
					Ports: map[string]*pbcatalog.WorkloadPort{
						"tcp1": {Port: 8080},
						"tcp2": {Port: 8081},
						"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
				{
					TargetRef: workload2.Id,
					Ports: map[string]*pbcatalog.WorkloadPort{
						"tcp1": {Port: 8080},
						"tcp2": {Port: 8081},
						"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
			},
		}).Build()
	proxyTmpl1ID := resourcetest.Resource(types.ProxyStateTemplateType, "workload-1").ID()
	proxyTmpl2ID := resourcetest.Resource(types.ProxyStateTemplateType, "workload-2").ID()

	c := sidecarproxycache.NewDestinationsCache()
	mapper := &Mapper{destinationsCache: c}
	sourceProxy1 := resourcetest.Resource(types.ProxyStateTemplateType, "workload-3").ID()
	sourceProxy2 := resourcetest.Resource(types.ProxyStateTemplateType, "workload-4").ID()
	sourceProxy3 := resourcetest.Resource(types.ProxyStateTemplateType, "workload-5").ID()
	destination1 := intermediate.CombinedDestinationRef{
		ServiceRef: resourcetest.Resource(catalog.ServiceType, "service").ReferenceNoSection(),
		Port:       "tcp1",
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(sourceProxy1): {},
			resource.NewReferenceKey(sourceProxy2): {},
		},
	}
	destination2 := intermediate.CombinedDestinationRef{
		ServiceRef: resourcetest.Resource(catalog.ServiceType, "service").ReferenceNoSection(),
		Port:       "tcp2",
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(sourceProxy1): {},
			resource.NewReferenceKey(sourceProxy3): {},
		},
	}
	destination3 := intermediate.CombinedDestinationRef{
		ServiceRef: resourcetest.Resource(catalog.ServiceType, "service").ReferenceNoSection(),
		Port:       "mesh",
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(sourceProxy1): {},
			resource.NewReferenceKey(sourceProxy3): {},
		},
	}
	c.WriteDestination(destination1)
	c.WriteDestination(destination2)
	c.WriteDestination(destination3)

	expRequests := []controller.Request{
		{ID: proxyTmpl1ID},
		{ID: proxyTmpl2ID},
		{ID: sourceProxy1},
		{ID: sourceProxy2},
		{ID: sourceProxy1},
		{ID: sourceProxy3},
	}

	requests, err := mapper.MapServiceEndpointsToProxyStateTemplate(context.Background(), controller.Runtime{Client: client}, serviceEndpoints)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, expRequests, requests)
}
