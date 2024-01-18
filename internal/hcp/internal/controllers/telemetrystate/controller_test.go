package telemetrystate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
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

	ctl       telemetryStateReconciler
	tenancies []*pbresource.Tenancy
}

func mockHcpClientFn(t *testing.T) (*hcpclient.MockClient, link.HCPClientFn) {
	mockClient := hcpclient.NewMockClient(t)

	mockClientFunc := func(link *pbhcp.Link) (hcpclient.Client, error) {
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

	suite.rt = controller.Runtime{
		Client: client,
		Logger: testutil.Logger(suite.T()),
	}
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
		Endpoint:      "test",
		MetricsConfig: &hcpclient.MetricsConfig{},
		RefreshConfig: &hcpclient.RefreshConfig{},
	}, nil)
	mockClient.EXPECT().GetObservabilitySecret(mock.Anything).Return("xxx", "yyy", nil)
	mgr.Register(TelemetryStateController(mockClientFn))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}

	link := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		WithStatus(link.StatusKey, &pbresource.Status{Conditions: []*pbresource.Condition{link.ConditionLinked(linkData.ResourceId)}}).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(link.Id))

	tsRes := suite.client.WaitForResourceExists(suite.T(), &pbresource.ID{Name: "global", Type: pbhcp.TelemetryStateType})
	decodedState, err := resource.Decode[*pbhcp.TelemetryState](tsRes)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), linkData.ResourceId, decodedState.GetData().ResourceId)
	require.Equal(suite.T(), "xxx", decodedState.GetData().ClientId)
	require.Equal(suite.T(), "test", decodedState.GetData().Endpoint)
}
