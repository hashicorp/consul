// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/multicluster"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/testing/golden"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	meshv2beta1 "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func protoToJSON(t *testing.T, pb proto.Message) string {
	return prototest.ProtoToJSON(t, pb)
}

type apiGatewayStateTemplateBuilderSuite struct {
	suite.Suite

	ctx            context.Context
	client         pbresource.ResourceServiceClient
	resourceClient *resourcetest.Client
	rt             controller.Runtime

	workload              *types.DecodedWorkload
	computedConfiguration *meshv2beta1.ComputedGatewayConfiguration

	tenancies []*pbresource.Tenancy
}

func (suite *apiGatewayStateTemplateBuilderSuite) SetupTest() {
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

func (suite *apiGatewayStateTemplateBuilderSuite) setupWithTenancy(tenancy *pbresource.Tenancy) {
	suite.workload = &types.DecodedWorkload{
		Data: &pbcatalog.Workload{
			Identity: "test",
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host:     "127.0.0.1",
					External: false,
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"tcp": {
					Port:     23,
					Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
				},
			},
		},
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    "test",
				Tenancy: tenancy,
			},
		},
	}

	suite.computedConfiguration = &pbmesh.ComputedGatewayConfiguration{
		ListenerConfigs: map[string]*pbmesh.ComputedGatewayListener{
			"tcp": &pbmesh.ComputedGatewayListener{
				HostnameConfigs: map[string]*pbmesh.ComputedHostnameRoutes{
					"*": &pbmesh.ComputedHostnameRoutes{
						Routes: &pbmesh.ComputedPortRoutes{
							Config: &pbmesh.ComputedPortRoutes_Tcp{
								Tcp: &pbmesh.ComputedTCPRoute{
									Rules: []*pbmesh.ComputedTCPRouteRule{
										{BackendRefs: []*pbmesh.ComputedTCPBackendRef{
											{BackendTarget: "backend-v1", Weight: 3},
											{BackendTarget: "backend-v2", Weight: 1},
										}},
									},
								},
							},
							ParentRef: &pbmesh.ParentReference{
								Ref: &pbresource.Reference{
									Type:    pbmesh.APIGatewayType,
									Tenancy: tenancy,
									Name:    "test",
								},
							},
							Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
							Targets: map[string]*pbmesh.BackendTargetDetails{
								"backend-v1": &pbmesh.BackendTargetDetails{
									Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
									DestinationConfig: &pbmesh.DestinationConfig{},
									ServiceEndpointsRef: &pbproxystate.EndpointRef{
										Id: &pbresource.ID{
											Name:    "backend-v1",
											Type:    pbcatalog.ServiceEndpointsType,
											Tenancy: tenancy,
										},
										RoutePort: "target",
										MeshPort:  "mesh",
									},
									BackendRef: &pbmesh.BackendReference{
										Ref: &pbresource.Reference{
											Tenancy: tenancy,
											Name:    "backend-v1",
										},
										Port:       "target",
										Datacenter: "dc1",
									},
								},
								"backend-v2": &pbmesh.BackendTargetDetails{
									Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
									DestinationConfig: &pbmesh.DestinationConfig{},
									ServiceEndpointsRef: &pbproxystate.EndpointRef{
										Id: &pbresource.ID{
											Name:    "backend-v2",
											Type:    pbcatalog.ServiceEndpointsType,
											Tenancy: tenancy,
										},
										RoutePort: "target",
										MeshPort:  "mesh",
									},
									BackendRef: &pbmesh.BackendReference{
										Ref: &pbresource.Reference{
											Tenancy: tenancy,
											Name:    "backend-v2",
										},
										Port:       "target",
										Datacenter: "dc1",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (suite *apiGatewayStateTemplateBuilderSuite) TestProxyStateTemplateBuilder_BuildForPeeredExportedServices() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		t := suite.T()

		logger := testutil.Logger(t)
		builder := NewAPIGWProxyStateTemplateBuilder(suite.workload, suite.computedConfiguration, logger, fetcher.New(suite.client), "dc1", "domain")

		actual := protoToJSON(t, builder.Build())
		expected := golden.Get(t, actual, "api-"+tenancy.Partition+"-"+tenancy.Namespace+".golden")
		require.JSONEq(t, expected, actual)
	})
}

func TestAPIGatewayStateTemplateBuilder(t *testing.T) {
	suite.Run(t, new(apiGatewayStateTemplateBuilderSuite))
}

func (suite *apiGatewayStateTemplateBuilderSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *apiGatewayStateTemplateBuilderSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupWithTenancy(tenancy)
			t(tenancy)
		})
	}
}
