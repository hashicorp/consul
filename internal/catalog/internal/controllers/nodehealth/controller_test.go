// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nodehealth

import (
	"context"
	"fmt"
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	nodeData = &pbcatalog.Node{
		Addresses: []*pbcatalog.NodeAddress{
			{
				Host: "127.0.0.1",
			},
		},
	}

	dnsPolicyData = &pbcatalog.DNSPolicy{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Weights: &pbcatalog.Weights{
			Passing: 1,
			Warning: 1,
		},
	}
)

func resourceID(rtype *pbresource.Type, name string) *pbresource.ID {
	// TODO: inject registration to get at scope or deal with the if stmt
	var tenancy *pbresource.Tenancy
	if rtype.Kind == types.NodeKind {
		tenancy = resource.DefaultPartitionedTenancy()
	} else {
		tenancy = resource.DefaultNamespacedTenancy()
	}

	return &pbresource.ID{
		Type:    rtype,
		Tenancy: tenancy,
		Name:    name,
	}
}

type nodeHealthControllerTestSuite struct {
	suite.Suite

	resourceClient pbresource.ResourceServiceClient
	runtime        controller.Runtime
	registry       resource.Registry

	ctl nodeHealthReconciler

	nodeNoHealth    *pbresource.ID
	nodePassing     *pbresource.ID
	nodeWarning     *pbresource.ID
	nodeCritical    *pbresource.ID
	nodeMaintenance *pbresource.ID
}

func (suite *nodeHealthControllerTestSuite) SetupTest() {
	suite.resourceClient, suite.registry = svctest.RunResourceService2(suite.T(), types.Register)
	suite.runtime = controller.Runtime{Client: suite.resourceClient, Logger: testutil.Logger(suite.T())}

	// The rest of the setup will be to prime the resource service with some data
	suite.nodeNoHealth = resourcetest.Resource(types.NodeType, "test-node-no-health").
		WithData(suite.T(), nodeData).
		Write(suite.T(), suite.resourceClient).Id

	suite.nodePassing = resourcetest.Resource(types.NodeType, "test-node-passing").
		WithData(suite.T(), nodeData).
		Write(suite.T(), suite.resourceClient).Id

	suite.nodeWarning = resourcetest.Resource(types.NodeType, "test-node-warning").
		WithData(suite.T(), nodeData).
		Write(suite.T(), suite.resourceClient).Id

	suite.nodeCritical = resourcetest.Resource(types.NodeType, "test-node-critical").
		WithData(suite.T(), nodeData).
		Write(suite.T(), suite.resourceClient).Id

	suite.nodeMaintenance = resourcetest.Resource(types.NodeType, "test-node-maintenance").
		WithData(suite.T(), nodeData).
		Write(suite.T(), suite.resourceClient).Id

	nodeHealthDesiredStatus := map[string]pbcatalog.Health{
		suite.nodePassing.Name:     pbcatalog.Health_HEALTH_PASSING,
		suite.nodeWarning.Name:     pbcatalog.Health_HEALTH_WARNING,
		suite.nodeCritical.Name:    pbcatalog.Health_HEALTH_CRITICAL,
		suite.nodeMaintenance.Name: pbcatalog.Health_HEALTH_MAINTENANCE,
	}

	// In order to exercise the behavior to ensure that its not a last-status-wins sort of thing
	// we are strategically naming health statuses so that they will be returned in an order with
	// the most precedent status being in the middle of the list. This will ensure that statuses
	// seen later can overide a previous status and that statuses seen later do not override if
	// they would lower the overall status such as going from critical -> warning.
	precedenceHealth := []pbcatalog.Health{
		pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_MAINTENANCE,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_PASSING,
	}

	for _, node := range []*pbresource.ID{suite.nodePassing, suite.nodeWarning, suite.nodeCritical, suite.nodeMaintenance} {
		for idx, health := range precedenceHealth {
			if nodeHealthDesiredStatus[node.Name] >= health {
				resourcetest.Resource(types.HealthStatusType, fmt.Sprintf("test-check-%s-%d", node.Name, idx)).
					WithData(suite.T(), &pbcatalog.HealthStatus{Type: "tcp", Status: health}).
					WithOwner(node).
					Write(suite.T(), suite.resourceClient)
			}
		}
	}

	// create a DNSPolicy to be owned by the node. The type doesn't really matter it just needs
	// to be something that doesn't care about its owner. All we want to prove is that we are
	// filtering out non-HealthStatus types appropriately.
	resourcetest.Resource(types.DNSPolicyType, "test-policy").
		WithData(suite.T(), dnsPolicyData).
		WithOwner(suite.nodeNoHealth).
		Write(suite.T(), suite.resourceClient)
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthListError() {
	// This resource id references a resource type that will not be
	// registered with the resource service. The ListByOwner call
	// should produce an InvalidArgument error. This test is meant
	// to validate how that error is handled (its propagated back
	// to the caller)
	ref := resourceID(
		&pbresource.Type{Group: "not", GroupVersion: "v1", Kind: "found"},
		"irrelevant",
	)
	health, err := getNodeHealth(context.Background(), suite.runtime, ref)
	require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	require.Error(suite.T(), err)
	require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthNoNode() {
	// This test is meant to ensure that when the node doesn't exist
	// no error is returned but also no data is. The default passing
	// status should then be returned in the same manner as the node
	// existing but with no associated HealthStatus resources.
	ref := resourceID(types.NodeType, "foo")
	ref.Uid = ulid.Make().String()
	health, err := getNodeHealth(context.Background(), suite.runtime, ref)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthNoStatus() {
	health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodeNoHealth)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthPassingStatus() {
	health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodePassing)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthCriticalStatus() {
	health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodeCritical)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthWarningStatus() {
	health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodeWarning)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), pbcatalog.Health_HEALTH_WARNING, health)
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthMaintenanceStatus() {
	health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodeMaintenance)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), pbcatalog.Health_HEALTH_MAINTENANCE, health)
}

func (suite *nodeHealthControllerTestSuite) TestReconcileNodeNotFound() {
	// This test ensures that removed nodes are ignored. In particular we don't
	// want to propagate the error and indefinitely keep re-reconciling in this case.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.NodeType, "not-found"),
	})
	require.NoError(suite.T(), err)
}

func (suite *nodeHealthControllerTestSuite) TestReconcilePropagateReadError() {
	// This test aims to ensure that errors other than NotFound errors coming
	// from the initial resource read get propagated. This case is very unrealistic
	// as the controller should not have given us a request ID for a resource type
	// that doesn't exist but this was the easiest way I could think of to synthesize
	// a Read error.
	ref := resourceID(
		&pbresource.Type{Group: "not", GroupVersion: "v1", Kind: "found"},
		"irrelevant",
	)

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: ref,
	})
	require.Error(suite.T(), err)
	require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
}

func (suite *nodeHealthControllerTestSuite) testReconcileStatus(id *pbresource.ID, expectedStatus *pbresource.Condition) *pbresource.Resource {
	suite.T().Helper()

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: id,
	})
	require.NoError(suite.T(), err)

	rsp, err := suite.resourceClient.Read(context.Background(), &pbresource.ReadRequest{
		Id: id,
	})
	require.NoError(suite.T(), err)

	nodeHealthStatus, found := rsp.Resource.Status[StatusKey]
	require.True(suite.T(), found)
	require.Equal(suite.T(), rsp.Resource.Generation, nodeHealthStatus.ObservedGeneration)
	require.Len(suite.T(), nodeHealthStatus.Conditions, 1)
	prototest.AssertDeepEqual(suite.T(),
		nodeHealthStatus.Conditions[0],
		expectedStatus)

	return rsp.Resource
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusPassing() {
	suite.testReconcileStatus(suite.nodePassing, &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  "HEALTH_PASSING",
		Message: NodeHealthyMessage,
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusWarning() {
	suite.testReconcileStatus(suite.nodeWarning, &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  "HEALTH_WARNING",
		Message: NodeUnhealthyMessage,
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusCritical() {
	suite.testReconcileStatus(suite.nodeCritical, &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  "HEALTH_CRITICAL",
		Message: NodeUnhealthyMessage,
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusMaintenance() {
	suite.testReconcileStatus(suite.nodeMaintenance, &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  "HEALTH_MAINTENANCE",
		Message: NodeUnhealthyMessage,
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_AvoidRereconciliationWrite() {
	res1 := suite.testReconcileStatus(suite.nodeWarning, &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  "HEALTH_WARNING",
		Message: NodeUnhealthyMessage,
	})

	res2 := suite.testReconcileStatus(suite.nodeWarning, &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  "HEALTH_WARNING",
		Message: NodeUnhealthyMessage,
	})

	// If another status write was performed then the versions would differ. This
	// therefore proves that after a second reconciliation without any change in status
	// that we are not making subsequent status writes.
	require.Equal(suite.T(), res1.Version, res2.Version)
}

func (suite *nodeHealthControllerTestSuite) waitForReconciliation(id *pbresource.ID, reason string) {
	suite.T().Helper()

	retry.Run(suite.T(), func(r *retry.R) {
		rsp, err := suite.resourceClient.Read(context.Background(), &pbresource.ReadRequest{
			Id: id,
		})
		require.NoError(r, err)

		nodeHealthStatus, found := rsp.Resource.Status[StatusKey]
		require.True(r, found)
		require.Equal(r, rsp.Resource.Generation, nodeHealthStatus.ObservedGeneration)
		require.Len(r, nodeHealthStatus.Conditions, 1)
		require.Equal(r, reason, nodeHealthStatus.Conditions[0].Reason)
	})
}
func (suite *nodeHealthControllerTestSuite) TestController() {
	// create the controller manager
	mgr := controller.NewManager(suite.resourceClient, suite.registry, testutil.Logger(suite.T()))

	// register our controller
	mgr.Register(NodeHealthController())
	mgr.SetRaftLeader(true)
	ctx, cancel := context.WithCancel(context.Background())
	suite.T().Cleanup(cancel)

	// run the manager
	go mgr.Run(ctx)

	// ensure that the node health eventually gets set.
	suite.waitForReconciliation(suite.nodePassing, "HEALTH_PASSING")

	// rewrite the resource - this will cause the nodes health
	// to be rereconciled but wont result in any health change
	resourcetest.Resource(types.NodeType, suite.nodePassing.Name).
		WithData(suite.T(), &pbcatalog.Node{
			Addresses: []*pbcatalog.NodeAddress{
				{
					Host: "198.18.0.1",
				},
			},
		}).
		Write(suite.T(), suite.resourceClient)

	// wait for rereconciliation to happen
	suite.waitForReconciliation(suite.nodePassing, "HEALTH_PASSING")

	resourcetest.Resource(types.HealthStatusType, "failure").
		WithData(suite.T(), &pbcatalog.HealthStatus{Type: "fake", Status: pbcatalog.Health_HEALTH_CRITICAL}).
		WithOwner(suite.nodePassing).
		Write(suite.T(), suite.resourceClient)

	suite.waitForReconciliation(suite.nodePassing, "HEALTH_CRITICAL")
}

func TestNodeHealthController(t *testing.T) {
	suite.Run(t, new(nodeHealthControllerTestSuite))
}
