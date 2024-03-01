// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apigateways

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
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type apigatewayControllerSuite struct {
	suite.Suite

	ctx            context.Context
	client         pbresource.ResourceServiceClient
	resourceClient *resourcetest.Client
	rt             controller.Runtime

	apiGateway *pbresource.Resource

	tenancies []*pbresource.Tenancy
}

func (suite *apigatewayControllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = resourcetest.TestTenancies()
	suite.client = svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())
	suite.resourceClient = resourcetest.NewClient(suite.client)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}
}

func (suite *apigatewayControllerSuite) TestReconciler_Reconcile() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		r := reconciler{}
		ctx := context.Background()

		testutil.RunStep(suite.T(), "api-gateway exists", func(t *testing.T) {
			id := &pbresource.ID{
				Name: "api-gateway",
				Type: &pbresource.Type{
					Group:        "mesh",
					GroupVersion: "v2beta1",
					Kind:         "APIGateway",
				},
				Tenancy: tenancy,
			}

			expectedWrittenService := &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-gateway"},
				},
				Ports: []*pbcatalog.ServicePort{
					{
						VirtualPort: 9090,
						TargetPort:  "http-listener",
						Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						VirtualPort: 8080,
						TargetPort:  "tcp-listener",
						Protocol:    pbcatalog.Protocol_PROTOCOL_TCP,
					},
					{
						VirtualPort: 8081,
						TargetPort:  "tcp-upper",
						Protocol:    pbcatalog.Protocol_PROTOCOL_TCP,
					},
				},
			}
			req := controller.Request{ID: id}
			err := r.Reconcile(ctx, suite.rt, req)

			require.NoError(t, err)

			dec, err := resource.GetDecodedResource[*pbcatalog.Service](ctx, suite.client, resource.ReplaceType(pbcatalog.ServiceType, req.ID))
			require.NoError(t, err)
			require.Equal(t, dec.Data.Ports, expectedWrittenService.Ports)
			require.Equal(t, dec.Data.Workloads, expectedWrittenService.Workloads)
		})

		testutil.RunStep(suite.T(), "api-gateway does not exist", func(t *testing.T) {
			id := &pbresource.ID{
				Name: "does-not-exist",
				Type: &pbresource.Type{
					Group:        "mesh",
					GroupVersion: "v2beta1",
					Kind:         "APIGateway",
				},
				Tenancy: tenancy,
			}

			req := controller.Request{ID: id}
			err := r.Reconcile(ctx, suite.rt, req)

			require.NoError(t, err)

			dec, err := resource.GetDecodedResource[*pbcatalog.Service](ctx, suite.client, resource.ReplaceType(pbcatalog.ServiceType, req.ID))
			require.NoError(t, err)
			require.Nil(t, dec)
		})
	})
}

func TestAPIGatewayReconciler(t *testing.T) {
	suite.Run(t, new(apigatewayControllerSuite))
}

func (suite *apigatewayControllerSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *apigatewayControllerSuite) setupSuiteWithTenancy(tenancy *pbresource.Tenancy) {
	suite.apiGateway = resourcetest.Resource(pbmesh.APIGatewayType, "api-gateway").
		WithData(suite.T(), &pbmesh.APIGateway{
			GatewayClassName: "consul",
			Listeners: []*pbmesh.APIGatewayListener{
				{
					Name:     "http-listener",
					Port:     9090,
					Protocol: "http",
				},
				{
					Name:     "tcp-listener",
					Port:     8080,
					Protocol: "tcp",
				},
				{
					Name:     "tcp-upper",
					Port:     8081,
					Protocol: "TCP",
				},
			},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)
}

func (suite *apigatewayControllerSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupSuiteWithTenancy(tenancy)
			t(tenancy)
		})
	}
}
