// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

type gatherWorkloadsDataSuite struct {
	suite.Suite

	ctx   context.Context
	cache cache.Cache

	apiServiceData       *pbcatalog.Service
	apiService           *resource.DecodedResource[*pbcatalog.Service]
	apiServiceSubsetData *pbcatalog.Service
	apiServiceSubset     *resource.DecodedResource[*pbcatalog.Service]
	apiEndpoints         *resource.DecodedResource[*pbcatalog.ServiceEndpoints]
	api1Workload         *resource.DecodedResource[*pbcatalog.Workload]
	api2Workload         *resource.DecodedResource[*pbcatalog.Workload]
	api123Workload       *resource.DecodedResource[*pbcatalog.Workload]
	web1Workload         *resource.DecodedResource[*pbcatalog.Workload]
	web2Workload         *resource.DecodedResource[*pbcatalog.Workload]

	tenancies []*pbresource.Tenancy
}

func (suite *gatherWorkloadsDataSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = rtest.TestTenancies()

	suite.cache = cache.New()
	suite.cache.AddType(pbcatalog.WorkloadType)

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

func (suite *gatherWorkloadsDataSuite) TestGetWorkloadData() {
	// This test's purpose is to ensure that gather workloads for
	// a service work as expected. The services selector was crafted
	// to exercise the deduplication behavior as well as the sorting
	// behavior. The assertions in this test will verify that only
	// unique workloads are returned and that they are ordered.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		require.NotNil(suite.T(), suite.apiService)

		data, err := GetWorkloadsWithSelector(suite.cache, suite.apiService)

		require.NoError(suite.T(), err)
		require.Len(suite.T(), data, 5)
		requireDecodedWorkloadEquals(suite.T(), suite.api1Workload, data[0])
		requireDecodedWorkloadEquals(suite.T(), suite.api1Workload, data[0])
		requireDecodedWorkloadEquals(suite.T(), suite.api123Workload, data[1])
		requireDecodedWorkloadEquals(suite.T(), suite.api2Workload, data[2])
		requireDecodedWorkloadEquals(suite.T(), suite.web1Workload, data[3])
		requireDecodedWorkloadEquals(suite.T(), suite.web2Workload, data[4])
	})
}

func (suite *gatherWorkloadsDataSuite) TestGetWorkloadDataWithFilter() {
	// This is like TestGetWorkloadData except it exercises the post-read
	// filter on the selector.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		require.NotNil(suite.T(), suite.apiServiceSubset)

		data, err := GetWorkloadsWithSelector(suite.cache, suite.apiServiceSubset)

		require.NoError(suite.T(), err)
		require.Len(suite.T(), data, 2)
		requireDecodedWorkloadEquals(suite.T(), suite.api123Workload, data[0])
		requireDecodedWorkloadEquals(suite.T(), suite.web1Workload, data[1])
	})
}

func TestReconciliationData(t *testing.T) {
	suite.Run(t, new(gatherWorkloadsDataSuite))
}

func (suite *gatherWorkloadsDataSuite) setupResourcesWithTenancy(tenancy *pbresource.Tenancy) {
	suite.apiService = rtest.MustDecode[*pbcatalog.Service](
		suite.T(),
		rtest.Resource(pbcatalog.ServiceType, "api").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.apiServiceData).
			Build())

	suite.apiServiceSubset = rtest.MustDecode[*pbcatalog.Service](
		suite.T(),
		rtest.Resource(pbcatalog.ServiceType, "api-subset").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.apiServiceSubsetData).
			Build())

	suite.api1Workload = rtest.MustDecode[*pbcatalog.Workload](
		suite.T(),
		rtest.Resource(pbcatalog.WorkloadType, "api-1").
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
			Build())
	suite.cache.Insert(suite.api1Workload.Resource)

	suite.api2Workload = rtest.MustDecode[*pbcatalog.Workload](
		suite.T(),
		rtest.Resource(pbcatalog.WorkloadType, "api-2").
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
			Build())
	suite.cache.Insert(suite.api2Workload.Resource)

	suite.api123Workload = rtest.MustDecode[*pbcatalog.Workload](
		suite.T(),
		rtest.Resource(pbcatalog.WorkloadType, "api-123").
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
			Build())
	suite.cache.Insert(suite.api123Workload.Resource)

	suite.web1Workload = rtest.MustDecode[*pbcatalog.Workload](
		suite.T(),
		rtest.Resource(pbcatalog.WorkloadType, "web-1").
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
			Build())
	suite.cache.Insert(suite.web1Workload.Resource)

	suite.web2Workload = rtest.MustDecode[*pbcatalog.Workload](
		suite.T(),
		rtest.Resource(pbcatalog.WorkloadType, "web-2").
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
			Build())
	suite.cache.Insert(suite.web2Workload.Resource)

	suite.apiEndpoints = rtest.MustDecode[*pbcatalog.ServiceEndpoints](
		suite.T(),
		rtest.Resource(pbcatalog.ServiceEndpointsType, "api").
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
			Build())
}

func (suite *gatherWorkloadsDataSuite) cleanupResources() {
	require.NoError(suite.T(), suite.cache.Delete(suite.api1Workload.Resource))
	require.NoError(suite.T(), suite.cache.Delete(suite.api2Workload.Resource))
	require.NoError(suite.T(), suite.cache.Delete(suite.api123Workload.Resource))
	require.NoError(suite.T(), suite.cache.Delete(suite.web1Workload.Resource))
	require.NoError(suite.T(), suite.cache.Delete(suite.web2Workload.Resource))
}

func (suite *gatherWorkloadsDataSuite) runTestCaseWithTenancies(testFunc func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupResourcesWithTenancy(tenancy)
			testFunc(tenancy)
			suite.T().Cleanup(suite.cleanupResources)
		})
	}
}

func (suite *gatherWorkloadsDataSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func requireDecodedWorkloadEquals(t testutil.TestingTB, expected, actual *resource.DecodedResource[*pbcatalog.Workload]) {
	prototest.AssertDeepEqual(t, expected.Resource, actual.Resource)
	require.Equal(t, expected.Data, actual.Data)
}
