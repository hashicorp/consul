package trafficpermissions

import (
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type controllerSuite struct {
	suite.Suite
	client  pbresource.ResourceServiceClient
	runtime controller.Runtime
}

func (suite *controllerSuite) SetupTest() {
	suite.client = svctest.RunResourceService(suite.T(), types.Register)
	suite.runtime = controller.Runtime{Client: suite.client, Logger: testutil.Logger(suite.T())}
}

type trafficPermissionsControllerSuite struct {
	controllerSuite

	mapper     *Mapper
	reconciler reconciler
}

func (suite *trafficPermissionsControllerSuite) testReconcileWithWorkload() {}

func (suite *trafficPermissionsControllerSuite) testReconcileWithTrafficPermission() {}
