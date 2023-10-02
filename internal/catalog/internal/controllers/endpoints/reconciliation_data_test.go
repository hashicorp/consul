// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

type reconciliationDataSuite struct {
	suite.Suite

	ctx    context.Context
	client pbresource.ResourceServiceClient
	rt     controller.Runtime

	apiServiceData       *pbcatalog.Service
	apiService           *pbresource.Resource
	apiServiceSubsetData *pbcatalog.Service
	apiServiceSubset     *pbresource.Resource
	apiEndpoints         *pbresource.Resource
	api1Workload         *pbresource.Resource
	api2Workload         *pbresource.Resource
	api123Workload       *pbresource.Resource
	web1Workload         *pbresource.Resource
	web2Workload         *pbresource.Resource
}

func (suite *reconciliationDataSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.client = svctest.RunResourceService(suite.T(), types.Register)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}

	suite.apiServiceData = &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			// This services selectors are specially crafted to exercise both the
			// dedeuplication and sorting behaviors of gatherWorkloadsForService
			Prefixes: []string{"api-"},
			Names:    []string{"api-1", "web-2", "web-1", "api-1", "not-found"},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "http",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
			},
		},
	}
	suite.apiServiceSubsetData = proto.Clone(suite.apiServiceData).(*pbcatalog.Service)
	suite.apiServiceSubsetData.Workloads.Filter = "(zim in metadata) and (metadata.zim matches `^g.`)"

	suite.apiService = rtest.Resource(pbcatalog.ServiceType, "api").
		WithData(suite.T(), suite.apiServiceData).
		Write(suite.T(), suite.client)

	suite.apiServiceSubset = rtest.Resource(pbcatalog.ServiceType, "api-subset").
		WithData(suite.T(), suite.apiServiceSubsetData).
		Write(suite.T(), suite.client)

	suite.api1Workload = rtest.Resource(pbcatalog.WorkloadType, "api-1").
		WithMeta("zim", "dib").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "api",
		}).
		Write(suite.T(), suite.client)

	suite.api2Workload = rtest.Resource(pbcatalog.WorkloadType, "api-2").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "api",
		}).
		Write(suite.T(), suite.client)

	suite.api123Workload = rtest.Resource(pbcatalog.WorkloadType, "api-123").
		WithMeta("zim", "gir").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "api",
		}).
		Write(suite.T(), suite.client)

	suite.web1Workload = rtest.Resource(pbcatalog.WorkloadType, "web-1").
		WithMeta("zim", "gaz").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "web",
		}).
		Write(suite.T(), suite.client)

	suite.web2Workload = rtest.Resource(pbcatalog.WorkloadType, "web-2").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "web",
		}).
		Write(suite.T(), suite.client)

	suite.apiEndpoints = rtest.Resource(pbcatalog.ServiceEndpointsType, "api").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: rtest.Resource(pbcatalog.WorkloadType, "api-1").WithTenancy(resource.DefaultNamespacedTenancy()).ID(),
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host:  "127.0.0.1",
							Ports: []string{"http"},
						},
					},
					Ports: map[string]*pbcatalog.WorkloadPort{
						"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					},
					HealthStatus: pbcatalog.Health_HEALTH_PASSING,
				},
			},
		}).
		Write(suite.T(), suite.client)
}

func (suite *reconciliationDataSuite) TestGetWorkloadData() {
	// This test's purpose is to ensure that gather workloads for
	// a service work as expected. The services selector was crafted
	// to exercise the deduplication behavior as well as the sorting
	// behavior. The assertions in this test will verify that only
	// unique workloads are returned and that they are ordered.

	data, err := getWorkloadData(suite.ctx, suite.rt, &serviceData{
		Resource: suite.apiService,
		Data:     suite.apiServiceData,
	})

	require.NoError(suite.T(), err)
	require.Len(suite.T(), data, 5)
	prototest.AssertDeepEqual(suite.T(), suite.api1Workload, data[0].Resource)
	prototest.AssertDeepEqual(suite.T(), suite.api123Workload, data[1].Resource)
	prototest.AssertDeepEqual(suite.T(), suite.api2Workload, data[2].Resource)
	prototest.AssertDeepEqual(suite.T(), suite.web1Workload, data[3].Resource)
	prototest.AssertDeepEqual(suite.T(), suite.web2Workload, data[4].Resource)
}

func (suite *reconciliationDataSuite) TestGetWorkloadDataWithFilter() {
	// This is like TestGetWorkloadData except it exercises the post-read
	// filter on the selector.
	data, err := getWorkloadData(suite.ctx, suite.rt, &serviceData{
		resource: suite.apiServiceSubset,
		service:  suite.apiServiceSubsetData,
	})

	require.NoError(suite.T(), err)
	require.Len(suite.T(), data, 2)
	prototest.AssertDeepEqual(suite.T(), suite.api123Workload, data[0].resource)
	prototest.AssertDeepEqual(suite.T(), suite.web1Workload, data[1].resource)
}

func TestReconciliationData(t *testing.T) {
	suite.Run(t, new(reconciliationDataSuite))
}
