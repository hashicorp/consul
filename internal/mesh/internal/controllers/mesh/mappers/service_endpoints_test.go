package mappers

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
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
				},
				{
					TargetRef: workload2.Id,
				},
			},
		}).Build()
	proxyTmpl1ID := resourcetest.Resource(types.ProxyStateTemplateType, "workload-1").ID()
	proxyTmpl2ID := resourcetest.Resource(types.ProxyStateTemplateType, "workload-2").ID()

	expRequests := []controller.Request{
		{
			ID: proxyTmpl1ID,
		},
		{
			ID: proxyTmpl2ID,
		},
	}

	requests, err := MapServiceEndpointsToProxyStateTemplate(context.Background(), controller.Runtime{}, serviceEndpoints)
	require.NoError(t, err)
	require.ElementsMatch(t, expRequests, requests)
}
