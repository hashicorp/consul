// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

type reconciliationDataSuite struct {
	suite.Suite

	ctx    context.Context
	client *resourcetest.Client
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

	tenancies []*pbresource.Tenancy
}

func (suite *reconciliationDataSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())

	suite.tenancies = rtest.TestTenancies()
	resourceClient := svctest.RunResourceServiceWithTenancies(suite.T(), types.Register)
	suite.client = resourcetest.NewClient(resourceClient)
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
}

func (suite *reconciliationDataSuite) TestGetServiceData_NotFound() {
	// This test's purposes is to ensure that NotFound errors when retrieving
	// the service data are ignored properly.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		data, err := getServiceData(suite.ctx, suite.rt, rtest.Resource(pbcatalog.ServiceType, "not-found").WithTenancy(tenancy).ID())
		require.NoError(suite.T(), err)
		require.Nil(suite.T(), data)
	})
}

func (suite *reconciliationDataSuite) TestGetServiceData_ReadError() {
	// This test's purpose is to ensure that Read errors other than NotFound
	// are propagated back to the caller. Specifying a resource ID with an
	// unregistered type is the easiest way to force a resource service error.
	badType := &pbresource.Type{
		Group:        "not",
		Kind:         "found",
		GroupVersion: "vfake",
	}
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		data, err := getServiceData(suite.ctx, suite.rt, rtest.Resource(badType, "foo").WithTenancy(tenancy).ID())
		require.Error(suite.T(), err)
		require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
		require.Nil(suite.T(), data)
	})
}

func (suite *reconciliationDataSuite) TestGetServiceData_UnmarshalError() {
	// This test's purpose is to ensure that unmarshlling errors are returned
	// to the caller. We are using a resource id that points to an endpoints
	// object instead of a service to ensure that the data will be unmarshallable.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		data, err := getServiceData(suite.ctx, suite.rt, rtest.Resource(pbcatalog.ServiceEndpointsType, "api").WithTenancy(tenancy).ID())
		require.Error(suite.T(), err)
		var parseErr resource.ErrDataParse
		require.ErrorAs(suite.T(), err, &parseErr)
		require.Nil(suite.T(), data)
	})
}

func (suite *reconciliationDataSuite) TestGetServiceData_Ok() {
	// This test's purpose is to ensure that the happy path for
	// retrieving a service works as expected.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		data, err := getServiceData(suite.ctx, suite.rt, suite.apiService.Id)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), data)
		require.NotNil(suite.T(), data.resource)
		prototest.AssertDeepEqual(suite.T(), suite.apiService.Id, data.resource.Id)
		require.Len(suite.T(), data.service.Ports, 1)
	})
}

func (suite *reconciliationDataSuite) TestGetEndpointsData_NotFound() {
	// This test's purposes is to ensure that NotFound errors when retrieving
	// the endpoint data are ignored properly.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		data, err := getEndpointsData(suite.ctx, suite.rt, rtest.Resource(pbcatalog.ServiceEndpointsType, "not-found").WithTenancy(tenancy).ID())
		require.NoError(suite.T(), err)
		require.Nil(suite.T(), data)
	})
}

func (suite *reconciliationDataSuite) TestGetEndpointsData_ReadError() {
	// This test's purpose is to ensure that Read errors other than NotFound
	// are propagated back to the caller. Specifying a resource ID with an
	// unregistered type is the easiest way to force a resource service error.
	badType := &pbresource.Type{
		Group:        "not",
		Kind:         "found",
		GroupVersion: "vfake",
	}
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		data, err := getEndpointsData(suite.ctx, suite.rt, rtest.Resource(badType, "foo").WithTenancy(tenancy).ID())
		require.Error(suite.T(), err)
		require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
		require.Nil(suite.T(), data)
	})
}

func (suite *reconciliationDataSuite) TestGetEndpointsData_UnmarshalError() {
	// This test's purpose is to ensure that unmarshlling errors are returned
	// to the caller. We are using a resource id that points to a service object
	// instead of an endpoints object to ensure that the data will be unmarshallable.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		data, err := getEndpointsData(suite.ctx, suite.rt, rtest.Resource(pbcatalog.ServiceType, "api").WithTenancy(tenancy).ID())
		require.Error(suite.T(), err)
		var parseErr resource.ErrDataParse
		require.ErrorAs(suite.T(), err, &parseErr)
		require.Nil(suite.T(), data)
	})
}

func (suite *reconciliationDataSuite) TestGetEndpointsData_Ok() {
	// This test's purpose is to ensure that the happy path for
	// retrieving an endpoints object works as expected.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		data, err := getEndpointsData(suite.ctx, suite.rt, suite.apiEndpoints.Id)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), data)
		require.NotNil(suite.T(), data.resource)
		prototest.AssertDeepEqual(suite.T(), suite.apiEndpoints.Id, data.resource.Id)
		require.Len(suite.T(), data.endpoints.Endpoints, 1)
	})
}

func (suite *reconciliationDataSuite) TestGetWorkloadData() {
	// This test's purpose is to ensure that gather workloads for
	// a service work as expected. The services selector was crafted
	// to exercise the deduplication behavior as well as the sorting
	// behavior. The assertions in this test will verify that only
	// unique workloads are returned and that they are ordered.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		require.NotNil(suite.T(), suite.apiService)

		data, err := getWorkloadData(suite.ctx, suite.rt, &serviceData{
			resource: suite.apiService,
			service:  suite.apiServiceData,
		})

		require.NoError(suite.T(), err)
		require.Len(suite.T(), data, 5)
		prototest.AssertDeepEqual(suite.T(), suite.api1Workload, data[0].resource)
		prototest.AssertDeepEqual(suite.T(), suite.api123Workload, data[1].resource)
		prototest.AssertDeepEqual(suite.T(), suite.api2Workload, data[2].resource)
		prototest.AssertDeepEqual(suite.T(), suite.web1Workload, data[3].resource)
		prototest.AssertDeepEqual(suite.T(), suite.web2Workload, data[4].resource)
	})
}

func (suite *reconciliationDataSuite) TestGetWorkloadDataWithFilter() {
	// This is like TestGetWorkloadData except it exercises the post-read
	// filter on the selector.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		require.NotNil(suite.T(), suite.apiServiceSubset)

		data, err := getWorkloadData(suite.ctx, suite.rt, &serviceData{
			resource: suite.apiServiceSubset,
			service:  suite.apiServiceSubsetData,
		})

		require.NoError(suite.T(), err)
		require.Len(suite.T(), data, 2)
		prototest.AssertDeepEqual(suite.T(), suite.api123Workload, data[0].resource)
		prototest.AssertDeepEqual(suite.T(), suite.web1Workload, data[1].resource)
	})
}

func TestReconciliationData(t *testing.T) {
	suite.Run(t, new(reconciliationDataSuite))
}

func (suite *reconciliationDataSuite) setupResourcesWithTenancy(tenancy *pbresource.Tenancy) {
	suite.apiService = rtest.Resource(pbcatalog.ServiceType, "api").
		WithTenancy(tenancy).
		WithData(suite.T(), suite.apiServiceData).
		Write(suite.T(), suite.client)

	suite.apiServiceSubset = rtest.Resource(pbcatalog.ServiceType, "api-subset").
		WithTenancy(tenancy).
		WithData(suite.T(), suite.apiServiceSubsetData).
		Write(suite.T(), suite.client)

	suite.api1Workload = rtest.Resource(pbcatalog.WorkloadType, "api-1").
		WithTenancy(tenancy).
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
		WithTenancy(tenancy).
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
		WithTenancy(tenancy).
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
		WithTenancy(tenancy).
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
		WithTenancy(tenancy).
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
		WithTenancy(tenancy).
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: rtest.Resource(pbcatalog.WorkloadType, "api-1").WithTenancy(tenancy).ID(),
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

func (suite *reconciliationDataSuite) cleanupResources() {
	suite.client.MustDelete(suite.T(), suite.apiService.Id)
	suite.client.MustDelete(suite.T(), suite.apiServiceSubset.Id)
	suite.client.MustDelete(suite.T(), suite.api1Workload.Id)
	suite.client.MustDelete(suite.T(), suite.api2Workload.Id)
	suite.client.MustDelete(suite.T(), suite.api123Workload.Id)
	suite.client.MustDelete(suite.T(), suite.web1Workload.Id)
	suite.client.MustDelete(suite.T(), suite.web2Workload.Id)
	suite.client.MustDelete(suite.T(), suite.apiEndpoints.Id)
}

func (suite *reconciliationDataSuite) runTestCaseWithTenancies(testFunc func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupResourcesWithTenancy(tenancy)
			testFunc(tenancy)
			suite.T().Cleanup(suite.cleanupResources)
		})
	}
}

func (suite *reconciliationDataSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}
