// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/multicluster"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type dataFetcherSuite struct {
	suite.Suite

	ctx            context.Context
	client         pbresource.ResourceServiceClient
	resourceClient *resourcetest.Client
	rt             controller.Runtime

	apiService         *pbresource.Resource
	meshGateway        *pbresource.Resource
	proxyStateTemplate *pbresource.Resource
	workload           *pbresource.Resource
	exportedServices   *pbresource.Resource

	tenancies []*pbresource.Tenancy
}

func (suite *dataFetcherSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = resourcetest.TestTenancies()
	suite.client = svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes, multicluster.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())
	suite.resourceClient = resourcetest.NewClient(suite.client)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}
}

func (suite *dataFetcherSuite) setupWithTenancy(tenancy *pbresource.Tenancy) {
	suite.apiService = resourcetest.Resource(pbcatalog.ServiceType, "api-1").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		},
		).
		Write(suite.T(), suite.client)

	suite.meshGateway = resourcetest.Resource(pbmesh.MeshGatewayType, "mesh-gateway").
		WithData(suite.T(), &pbmesh.MeshGateway{
			GatewayClassName: "gateway-class-1",
			Listeners: []*pbmesh.MeshGatewayListener{
				{
					Name: "wan",
				},
			},
		}).
		Write(suite.T(), suite.client)

	suite.proxyStateTemplate = resourcetest.Resource(pbmesh.ProxyStateTemplateType, "proxy-state-template-1").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{}).
		Write(suite.T(), suite.client)

	identityID := resourcetest.Resource(pbauth.WorkloadIdentityType, "workload-identity-abc").
		WithTenancy(tenancy).ID()

	suite.workload = resourcetest.Resource(pbcatalog.WorkloadType, "service-workload-abc").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbcatalog.Workload{
			Identity: identityID.Name,
			Ports: map[string]*pbcatalog.WorkloadPort{
				"foo": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host:  "10.0.0.1",
					Ports: []string{"foo"},
				},
			},
		}).Write(suite.T(), suite.client)

	suite.exportedServices = resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, "global").
		WithData(suite.T(), &pbmulticluster.ComputedExportedServices{}).
		Write(suite.T(), suite.client)
}

func (suite *dataFetcherSuite) TestFetcher_FetchMeshGateway() {
	suite.runTestCaseWithTenancies(func(_ *pbresource.Tenancy) {
		f := Fetcher{
			client: suite.client,
		}

		testutil.RunStep(suite.T(), "mesh gateway does not exist", func(t *testing.T) {
			nonExistantID := resourcetest.Resource(pbmesh.MeshGatewayType, "not-found").ID()
			gtw, err := f.FetchMeshGateway(suite.ctx, nonExistantID)
			require.NoError(t, err)
			require.Nil(t, gtw)
		})

		testutil.RunStep(suite.T(), "mesh gateway exists", func(t *testing.T) {
			gtw, err := f.FetchMeshGateway(suite.ctx, suite.meshGateway.Id)
			require.NoError(t, err)
			require.NotNil(t, gtw)
		})

		testutil.RunStep(suite.T(), "incorrect type is passed", func(t *testing.T) {
			incorrectID := resourcetest.Resource(pbcatalog.ServiceType, "api-1").ID()
			defer func() {
				err := recover()
				require.NotNil(t, err)
			}()
			f.FetchMeshGateway(suite.ctx, incorrectID)
		})
	})
}

func (suite *dataFetcherSuite) TestFetcher_FetchProxyStateTemplate() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		f := Fetcher{
			client: suite.client,
		}

		testutil.RunStep(suite.T(), "proxy state template does not exist", func(t *testing.T) {
			nonExistantID := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "not-found").WithTenancy(tenancy).ID()
			tmpl, err := f.FetchProxyStateTemplate(suite.ctx, nonExistantID)
			require.NoError(t, err)
			require.Nil(t, tmpl)
		})

		testutil.RunStep(suite.T(), "proxy state template exists", func(t *testing.T) {
			tmpl, err := f.FetchProxyStateTemplate(suite.ctx, suite.proxyStateTemplate.Id)
			require.NoError(t, err)
			require.NotNil(t, tmpl)
		})

		testutil.RunStep(suite.T(), "incorrect type is passed", func(t *testing.T) {
			incorrectID := resourcetest.Resource(pbcatalog.ServiceType, "api-1").ID()
			defer func() {
				err := recover()
				require.NotNil(t, err)
			}()
			f.FetchProxyStateTemplate(suite.ctx, incorrectID)
		})
	})
}

func (suite *dataFetcherSuite) TestFetcher_FetchWorkload() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		f := Fetcher{
			client: suite.client,
		}

		testutil.RunStep(suite.T(), "workload does not exist", func(t *testing.T) {
			nonExistantID := resourcetest.Resource(pbcatalog.WorkloadType, "not-found").WithTenancy(tenancy).ID()
			tmpl, err := f.FetchWorkload(suite.ctx, nonExistantID)
			require.NoError(t, err)
			require.Nil(t, tmpl)
		})

		testutil.RunStep(suite.T(), "workload exists", func(t *testing.T) {
			tmpl, err := f.FetchWorkload(suite.ctx, suite.workload.Id)
			require.NoError(t, err)
			require.NotNil(t, tmpl)
		})
	})
}

func (suite *dataFetcherSuite) TestFetcher_FetchExportedServices() {
	suite.runTestCaseWithTenancies(func(_ *pbresource.Tenancy) {
		f := Fetcher{
			client: suite.client,
		}

		testutil.RunStep(suite.T(), "exported services do not exist", func(t *testing.T) {
			nonExistantID := resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, "not-found").ID()
			svcs, err := f.FetchComputedExportedServices(suite.ctx, nonExistantID)
			require.NoError(t, err)
			require.Nil(t, svcs)
		})

		testutil.RunStep(suite.T(), "workload exists", func(t *testing.T) {
			svcs, err := f.FetchComputedExportedServices(suite.ctx, suite.exportedServices.Id)
			require.NoError(t, err)
			require.NotNil(t, svcs)
		})
	})
}

func (suite *dataFetcherSuite) TestFetcher_FetchService() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		f := Fetcher{
			client: suite.client,
		}

		testutil.RunStep(suite.T(), "service does not exist", func(t *testing.T) {
			nonExistantID := resourcetest.Resource(pbcatalog.ServiceType, "not-found").WithTenancy(tenancy).ID()
			svc, err := f.FetchService(suite.ctx, nonExistantID)
			require.NoError(t, err)
			require.Nil(t, svc)
		})

		testutil.RunStep(suite.T(), "service exists", func(t *testing.T) {
			svc, err := f.FetchService(suite.ctx, suite.apiService.Id)
			require.NoError(t, err)
			require.NotNil(t, svc)
		})

		testutil.RunStep(suite.T(), "incorrect type is passed", func(t *testing.T) {
			incorrectID := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-1").ID()
			defer func() {
				err := recover()
				require.NotNil(t, err)
			}()
			f.FetchService(suite.ctx, incorrectID)
		})
	})
}

func TestDataFetcher(t *testing.T) {
	suite.Run(t, new(dataFetcherSuite))
}

func (suite *dataFetcherSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *dataFetcherSuite) cleanUpNodes() {
	suite.resourceClient.MustDelete(suite.T(), suite.apiService.Id)
	suite.resourceClient.MustDelete(suite.T(), suite.meshGateway.Id)
	suite.resourceClient.MustDelete(suite.T(), suite.proxyStateTemplate.Id)
}

func (suite *dataFetcherSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupWithTenancy(tenancy)
			suite.T().Cleanup(func() {
				suite.cleanUpNodes()
			})
			t(tenancy)
		})
	}
}
