package hcclink

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type controllerSuite struct {
	suite.Suite

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	ctl hccLinkReconciler
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.RunResourceService(suite.T(), types.Register)
	suite.rt = controller.Runtime{
		Client: client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.client = rtest.NewClient(client)
}

func (suite *controllerSuite) TestController() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(HCCLinkController())
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	hccLinkData := &pbhcp.HCCLink{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}
	// The controller is currently a no-op, so there is nothing to test other than making sure we do not panic
	hccLink := rtest.Resource(pbhcp.HCCLinkType, "global").
		WithData(suite.T(), hccLinkData).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(hccLink.Id))
}

func TestHCCLinkController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func (suite *controllerSuite) deleteResourceFunc(id *pbresource.ID) func() {
	return func() {
		suite.client.MustDelete(suite.T(), id)
	}
}
