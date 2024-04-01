// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nodehealth

import (
	"context"
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/version/versiontest"
)

var (
	nodeData = &pbcatalog.Node{
		Addresses: []*pbcatalog.NodeAddress{
			{
				Host: "127.0.0.1",
			},
		},
	}

	workloadData = &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "127.0.0.1",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": {
				Port:     8443,
				Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2,
			},
		},
		NodeName: "foo",
		Identity: "api",
		Locality: &pbcatalog.Locality{
			Region: "us-east-1",
			Zone:   "1a",
		},
	}
)

func resourceID(rtype *pbresource.Type, name string, tenancy *pbresource.Tenancy) *pbresource.ID {
	return &pbresource.ID{
		Type:    rtype,
		Tenancy: tenancy,
		Name:    name,
	}
}

type nodeHealthControllerTestSuite struct {
	suite.Suite

	resourceClient *resourcetest.Client
	runtime        controller.Runtime

	ctl *controller.TestController

	nodeNoHealth    *pbresource.ID
	nodePassing     *pbresource.ID
	nodeWarning     *pbresource.ID
	nodeCritical    *pbresource.ID
	nodeMaintenance *pbresource.ID
	isEnterprise    bool
	tenancies       []*pbresource.Tenancy
}

func (suite *nodeHealthControllerTestSuite) writeNode(name string, tenancy *pbresource.Tenancy) *pbresource.ID {
	return resourcetest.Resource(pbcatalog.NodeType, name).
		WithData(suite.T(), nodeData).
		WithTenancy(&pbresource.Tenancy{
			Partition: tenancy.Partition,
		}).
		Write(suite.T(), suite.resourceClient).Id
}

func (suite *nodeHealthControllerTestSuite) SetupTest() {
	suite.tenancies = resourcetest.TestTenancies()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	suite.ctl = controller.NewTestController(NodeHealthController(), client).
		WithLogger(testutil.Logger(suite.T()))
	suite.runtime = suite.ctl.Runtime()
	suite.resourceClient = resourcetest.NewClient(suite.runtime.Client)
	suite.isEnterprise = versiontest.IsEnterprise()
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthNoNode() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// This test is meant to ensure that when the node doesn't exist
		// no error is returned but also no data is. The default passing
		// status should then be returned in the same manner as the node
		// existing but with no associated HealthStatus resources.
		ref := resourceID(pbcatalog.NodeType, "foo", &pbresource.Tenancy{
			Partition: tenancy.Partition,
		})
		ref.Uid = ulid.Make().String()
		health, err := getNodeHealth(suite.runtime, ref)

		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthNoStatus() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		health, err := getNodeHealth(suite.runtime, suite.nodeNoHealth)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthPassingStatus() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		health, err := getNodeHealth(suite.runtime, suite.nodePassing)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthCriticalStatus() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		health, err := getNodeHealth(suite.runtime, suite.nodeCritical)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthWarningStatus() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		health, err := getNodeHealth(suite.runtime, suite.nodeWarning)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_WARNING, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthMaintenanceStatus() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		health, err := getNodeHealth(suite.runtime, suite.nodeMaintenance)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_MAINTENANCE, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcileNodeNotFound() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// This test ensures that removed nodes are ignored. In particular we don't
		// want to propagate the error and indefinitely keep re-reconciling in this case.
		err := suite.ctl.Reconcile(context.Background(), controller.Request{
			ID: resourceID(pbcatalog.NodeType, "not-found", &pbresource.Tenancy{
				Partition: tenancy.Partition,
			}),
		})
		require.NoError(suite.T(), err)
	})
}

func (suite *nodeHealthControllerTestSuite) testReconcileStatus(id *pbresource.ID, expectedStatus *pbresource.Condition) *pbresource.Resource {
	suite.T().Helper()

	err := suite.ctl.Reconcile(context.Background(), controller.Request{
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
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		suite.testReconcileStatus(suite.nodePassing, &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_TRUE,
			Reason:  "HEALTH_PASSING",
			Message: NodeHealthyMessage,
		})
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusWarning() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		suite.testReconcileStatus(suite.nodeWarning, &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_FALSE,
			Reason:  "HEALTH_WARNING",
			Message: NodeUnhealthyMessage,
		})
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusCritical() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		suite.testReconcileStatus(suite.nodeCritical, &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_FALSE,
			Reason:  "HEALTH_CRITICAL",
			Message: NodeUnhealthyMessage,
		})
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusMaintenance() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		suite.testReconcileStatus(suite.nodeMaintenance, &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_FALSE,
			Reason:  "HEALTH_MAINTENANCE",
			Message: NodeUnhealthyMessage,
		})
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_AvoidRereconciliationWrite() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

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
	})
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
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		// create the controller manager
		mgr := controller.NewManager(suite.resourceClient, testutil.Logger(suite.T()))

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
		resourcetest.Resource(pbcatalog.NodeType, suite.nodePassing.Name).
			WithData(suite.T(), &pbcatalog.Node{
				Addresses: []*pbcatalog.NodeAddress{
					{
						Host: "198.18.0.1",
					},
				},
			}).
			WithTenancy(&pbresource.Tenancy{
				Partition: tenancy.Partition,
			}).
			Write(suite.T(), suite.resourceClient)

		// wait for rereconciliation to happen
		suite.waitForReconciliation(suite.nodePassing, "HEALTH_PASSING")

		resourcetest.Resource(pbcatalog.NodeHealthStatusType, "failure").
			WithData(suite.T(), &pbcatalog.NodeHealthStatus{Type: "fake", Status: pbcatalog.Health_HEALTH_CRITICAL}).
			WithOwner(suite.nodePassing).
			WithTenancy(&pbresource.Tenancy{
				Partition: tenancy.Partition,
			}).
			Write(suite.T(), suite.resourceClient)

		suite.waitForReconciliation(suite.nodePassing, "HEALTH_CRITICAL")
	})
}

func TestNodeHealthController(t *testing.T) {
	suite.Run(t, new(nodeHealthControllerTestSuite))
}

func (suite *nodeHealthControllerTestSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *nodeHealthControllerTestSuite) setupNodesWithTenancy(tenancy *pbresource.Tenancy) {

	// The rest of the setup will be to prime the resource service with some data
	suite.nodeNoHealth = suite.writeNode("test-node-no-health", tenancy)
	suite.nodePassing = suite.writeNode("test-node-passing", tenancy)
	suite.nodeWarning = suite.writeNode("test-node-warning", tenancy)
	suite.nodeCritical = suite.writeNode("test-node-critical", tenancy)
	suite.nodeMaintenance = suite.writeNode("test-node-maintenance", tenancy)

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
				resourcetest.Resource(pbcatalog.NodeHealthStatusType, fmt.Sprintf("test-check-%s-%d-%s", node.Name, idx, tenancy.Partition)).
					WithData(suite.T(), &pbcatalog.NodeHealthStatus{Type: "tcp", Status: health}).
					WithOwner(node).
					Write(suite.T(), suite.resourceClient)
			}
		}
	}

	// create a Workload to be owned by the node. The type doesn't really matter it just needs
	// to be something that doesn't care about its owner. All we want to prove is that we are
	// filtering out non-NodeHealthStatus types appropriately.
	resourcetest.Resource(pbcatalog.WorkloadType, "test-workload-"+tenancy.Partition+"-"+tenancy.Namespace).
		WithData(suite.T(), workloadData).
		WithOwner(suite.nodeNoHealth).
		WithTenancy(tenancy).
		Write(suite.T(), suite.resourceClient)
}

func (suite *nodeHealthControllerTestSuite) cleanUpNodes() {
	suite.resourceClient.MustDelete(suite.T(), suite.nodeNoHealth)
	suite.resourceClient.MustDelete(suite.T(), suite.nodeCritical)
	suite.resourceClient.MustDelete(suite.T(), suite.nodeWarning)
	suite.resourceClient.MustDelete(suite.T(), suite.nodePassing)
	suite.resourceClient.MustDelete(suite.T(), suite.nodeMaintenance)
}

func (suite *nodeHealthControllerTestSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupNodesWithTenancy(tenancy)
			suite.T().Cleanup(func() {
				suite.cleanUpNodes()
			})
			t(tenancy)
		})
	}
}
