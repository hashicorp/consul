// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package telemetrystate

import (
	"context"
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/link"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type controllerSuite struct {
	suite.Suite

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	ctl       *controller.TestController
	tenancies []*pbresource.Tenancy

	hcpMock *hcpclient.MockClient
}

func mockHcpClientFn(t *testing.T) (*hcpclient.MockClient, link.HCPClientFn) {
	mockClient := hcpclient.NewMockClient(t)

	mockClientFunc := func(link config.CloudConfig) (hcpclient.Client, error) {
		return mockClient, nil
	}

	return mockClient, mockClientFunc
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = rtest.TestTenancies()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	hcpMock, hcpClientFn := mockHcpClientFn(suite.T())
	suite.hcpMock = hcpMock
	suite.ctl = controller.NewTestController(TelemetryStateController(hcpClientFn), client).
		WithLogger(testutil.Logger(suite.T()))

	suite.rt = suite.ctl.Runtime()
	suite.client = rtest.NewClient(client)
}

func TestTelemetryStateController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func (suite *controllerSuite) deleteResourceFunc(id *pbresource.ID) func() {
	return func() {
		suite.client.MustDelete(suite.T(), id)
	}
}

func (suite *controllerSuite) TestController_Ok() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mockClient, mockClientFn := mockHcpClientFn(suite.T())
	mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&hcpclient.TelemetryConfig{
		MetricsConfig: &hcpclient.MetricsConfig{
			Endpoint: &url.URL{
				Scheme: "http",
				Host:   "localhost",
				Path:   "/test",
			},
			Labels:  map[string]string{"foo": "bar"},
			Filters: regexp.MustCompile(".*"),
		},
		RefreshConfig: &hcpclient.RefreshConfig{},
	}, nil)
	mockClient.EXPECT().GetObservabilitySecret(mock.Anything).Return("xxx", "yyy", nil)
	mgr.Register(TelemetryStateController(mockClientFn))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	link := suite.writeLinkResource()

	tsRes := suite.client.WaitForResourceExists(suite.T(), &pbresource.ID{Name: "global", Type: pbhcp.TelemetryStateType})
	decodedState, err := resource.Decode[*pbhcp.TelemetryState](tsRes)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), link.GetData().GetResourceId(), decodedState.GetData().ResourceId)
	require.Equal(suite.T(), "xxx", decodedState.GetData().ClientId)
	require.Equal(suite.T(), "http://localhost/test", decodedState.GetData().Metrics.Endpoint)

	suite.client.MustDelete(suite.T(), link.Id)
	suite.client.WaitForDeletion(suite.T(), tsRes.Id)
}

func (suite *controllerSuite) TestReconcile_AvoidReconciliationWriteLoop() {
	suite.hcpMock.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&hcpclient.TelemetryConfig{
		MetricsConfig: &hcpclient.MetricsConfig{
			Endpoint: &url.URL{
				Scheme: "http",
				Host:   "localhost",
				Path:   "/test",
			},
			Labels:  map[string]string{"foo": "bar"},
			Filters: regexp.MustCompile(".*"),
		},
		RefreshConfig: &hcpclient.RefreshConfig{},
	}, nil)
	link := suite.writeLinkResource()
	suite.hcpMock.EXPECT().GetObservabilitySecret(mock.Anything).Return("xxx", "yyy", nil)
	suite.NoError(suite.ctl.Reconcile(context.Background(), controller.Request{ID: link.Id}))
	tsRes := suite.client.WaitForResourceExists(suite.T(), &pbresource.ID{Name: "global", Type: pbhcp.TelemetryStateType})
	suite.NoError(suite.ctl.Reconcile(context.Background(), controller.Request{ID: tsRes.Id}))
	suite.client.RequireVersionUnchanged(suite.T(), tsRes.Id, tsRes.Version)
}

func (suite *controllerSuite) TestController_LinkingDisabled() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	_, mockClientFn := mockHcpClientFn(suite.T())
	mgr.Register(TelemetryStateController(mockClientFn))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   types.GenerateTestResourceID(suite.T()),
	}

	rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		WithStatus(link.StatusKey, &pbresource.Status{Conditions: []*pbresource.Condition{link.ConditionDisabled}}).
		Write(suite.T(), suite.client)

	suite.client.WaitForDeletion(suite.T(), &pbresource.ID{Name: "global", Type: pbhcp.TelemetryStateType})
}

func (suite *controllerSuite) writeLinkResource() *types.DecodedLink {
	suite.T().Helper()

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   types.GenerateTestResourceID(suite.T()),
	}

	res := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		WithStatus(link.StatusKey, &pbresource.Status{Conditions: []*pbresource.Condition{link.ConditionLinked(linkData.ResourceId)}}).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(res.Id))
	link, err := resource.Decode[*pbhcp.Link](res)
	require.NoError(suite.T(), err)
	return link
}
