package trafficpermissions

import (
	"context"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
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

func (suite *trafficPermissionsControllerSuite) testReconcileWithWorkload(workloadHealth pbcatalog.Health, status *pbresource.Condition) {
	suite.T().Helper()

	workload := resourcetest.Resource(types.WorkloadIdentityType, "test-workload").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host: "198.18.0.1",
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {
					Port:     8080,
					Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
				},
			},
			// I am guessing this will need to be wired up to workloadIdentity instead
			// of string eventually
			Identity: "test",
			NodeName: "test-node",
		}).Write(suite.T(), suite.client)

	err := suite.reconciler.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: workload.Id,
	})

	require.NoError(suite.T(), err)

	// ensure that workload is now being tracked by mapper
	reqs, err := suite.mapper.MapWorkloadToComputedTrafficPermission(context.Background(), suite.runtime, workload)
	require.NoError(suite.T(), err)
	//require.Len(suite.T(), reqs, 1)
	//prototest.AssertDeepEqual(suite.T(), reqs[0].ID, workload.Id)

	suite.T().Cleanup(func() {
		suite.mapper.UntrackWorkload(workload.Id)
	})

	return suite.checkWorkloadStatus(workload.Id, status)
}

// checkWorkloadStatus will read the workload resource and verify that its
// status has the expected value.
func (suite *trafficPermissionsControllerSuite) checkWorkloadStatus(id *pbresource.ID, status *pbresource.Condition) *pbresource.Resource {
	suite.T().Helper()

	rsp, err := suite.client.Read(context.Background(), &pbresource.ReadRequest{
		Id: id,
	})

	require.NoError(suite.T(), err)

	actualStatus, found := rsp.Resource.Status[StatusKey]
	require.True(suite.T(), found)
	require.Equal(suite.T(), rsp.Resource.Generation, actualStatus.ObservedGeneration)
	require.Len(suite.T(), actualStatus.Conditions, 1)
	prototest.AssertDeepEqual(suite.T(), status, actualStatus.Conditions[0])

	return rsp.Resource
}
