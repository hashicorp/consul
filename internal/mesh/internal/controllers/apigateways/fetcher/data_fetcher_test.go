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
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type dataFetcherSuite struct {
	suite.Suite

	ctx            context.Context
	client         pbresource.ResourceServiceClient
	resourceClient *resourcetest.Client
	rt             controller.Runtime

	apiGateway *pbresource.Resource

	tenancies []*pbresource.Tenancy
}

func (suite *dataFetcherSuite) SetupTest() {
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

func (suite *dataFetcherSuite) setupWithTenancy(tenancy *pbresource.Tenancy) {
	suite.apiGateway = resourcetest.Resource(pbmesh.APIGatewayType, "apigw").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbmesh.APIGateway{
			GatewayClassName: "consul",
			Listeners:        []*pbmesh.APIGatewayListener{},
		}).
		Write(suite.T(), suite.client)
}

func (suite *dataFetcherSuite) TestFetcher_FetchAPIGateway() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		f := Fetcher{
			client: suite.client,
		}

		testutil.RunStep(suite.T(), "gateway does not exist", func(t *testing.T) {
			nonExistantID := resourcetest.Resource(pbmesh.APIGatewayType, "not-found").WithTenancy(tenancy).ID()
			svc, err := f.FetchAPIGateway(suite.ctx, nonExistantID)
			require.NoError(t, err)
			require.Nil(t, svc)
		})

		testutil.RunStep(suite.T(), "gateway exists", func(t *testing.T) {
			svc, err := f.FetchAPIGateway(suite.ctx, suite.apiGateway.Id)
			require.NoError(t, err)
			require.NotNil(t, svc)
		})

		testutil.RunStep(suite.T(), "incorrect type is passed", func(t *testing.T) {
			incorrectID := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-1").ID()
			defer func() {
				err := recover()
				require.NotNil(t, err)
			}()
			f.FetchAPIGateway(suite.ctx, incorrectID)
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
	suite.resourceClient.MustDelete(suite.T(), suite.apiGateway.Id)
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
