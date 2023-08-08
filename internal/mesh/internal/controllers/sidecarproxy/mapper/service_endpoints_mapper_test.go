package mapper

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestMapServiceEndpointsToProxyStateTemplate(t *testing.T) {
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
					},
				},
				{
					TargetRef: workload2.Id,
					Ports: map[string]*pbcatalog.WorkloadPort{
						"tcp1": {Port: 8080},
						"tcp2": {Port: 8081},
					},
				},
			},
		}).Build()
	proxyTmpl1ID := resourcetest.Resource(types.ProxyStateTemplateType, "workload-1").ID()
	proxyTmpl2ID := resourcetest.Resource(types.ProxyStateTemplateType, "workload-2").ID()

	c := cache.New()
	mapper := &Mapper{cache: c}
	sourceProxy1 := resourcetest.Resource(types.ProxyStateTemplateType, "workload-3").ID()
	sourceProxy2 := resourcetest.Resource(types.ProxyStateTemplateType, "workload-4").ID()
	sourceProxy3 := resourcetest.Resource(types.ProxyStateTemplateType, "workload-5").ID()
	destination1 := &intermediate.CombinedDestinationRef{
		ServiceRef: resourcetest.Resource(catalog.ServiceType, "service").ReferenceNoSection(),
		Port:       "tcp1",
		SourceProxies: map[string]*pbresource.ID{
			cache.KeyFromID(sourceProxy1): sourceProxy1,
			cache.KeyFromID(sourceProxy2): sourceProxy2,
		},
	}
	destination2 := &intermediate.CombinedDestinationRef{
		ServiceRef: resourcetest.Resource(catalog.ServiceType, "service").ReferenceNoSection(),
		Port:       "tcp2",
		SourceProxies: map[string]*pbresource.ID{
			cache.KeyFromID(sourceProxy1): sourceProxy1,
			cache.KeyFromID(sourceProxy3): sourceProxy3,
		},
	}
	c.Write(destination1)
	c.Write(destination2)

	expRequests := []controller.Request{
		{ID: proxyTmpl1ID},
		{ID: proxyTmpl2ID},
		{ID: sourceProxy1},
		{ID: sourceProxy2},
		{ID: sourceProxy1},
		{ID: sourceProxy3},
	}

	requests, err := mapper.MapServiceEndpointsToProxyStateTemplate(context.Background(), controller.Runtime{}, serviceEndpoints)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, expRequests, requests)
}
