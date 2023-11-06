// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nodehealth

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/agent/structs"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mockres "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
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

func resourceID(rtype *pbresource.Type, name string, tenancy *pbresource.Tenancy) *pbresource.ID {
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

	ctl nodeHealthReconciler

	nodeNoHealth    map[string]*pbresource.ID
	nodePassing     map[string]*pbresource.ID
	nodeWarning     map[string]*pbresource.ID
	nodeCritical    map[string]*pbresource.ID
	nodeMaintenance map[string]*pbresource.ID
	isEnterprise    bool
	tenancies       []*pbresource.Tenancy
}

func (suite *nodeHealthControllerTestSuite) writeNode(name string, tenancy *pbresource.Tenancy) *pbresource.ID {
	return resourcetest.Resource(pbcatalog.NodeType, name).
		WithData(suite.T(), nodeData).
		WithTenancy(tenancy).
		Write(suite.T(), suite.resourceClient).Id
}

func (suite *nodeHealthControllerTestSuite) SetupTest() {
	mockTenancyBridge := &mockres.MockTenancyBridge{}
	suite.tenancies = resourcetest.TestTenancies()
	for _, tenancy := range suite.tenancies {
		mockTenancyBridge.On("PartitionExists", tenancy.Partition).Return(true, nil)
		mockTenancyBridge.On("NamespaceExists", tenancy.Partition, tenancy.Namespace).Return(true, nil)
		mockTenancyBridge.On("IsPartitionMarkedForDeletion", tenancy.Partition).Return(false, nil)
		mockTenancyBridge.On("IsNamespaceMarkedForDeletion", tenancy.Partition, tenancy.Namespace).Return(false, nil)
	}
	cfg := mockres.Config{
		TenancyBridge: mockTenancyBridge,
	}
	suite.resourceClient = svctest.RunResourceServiceWithConfig(suite.T(), cfg, types.Register, types.RegisterDNSPolicy)
	suite.runtime = controller.Runtime{Client: suite.resourceClient, Logger: testutil.Logger(suite.T())}
	suite.isEnterprise = structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty() == "default"
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthListError() {
	suite.runTestCaseWithTenancies("TestGetNodeHealthListError", func(tenancy *pbresource.Tenancy) {
		// This resource id references a resource type that will not be
		// registered with the resource service. The ListByOwner call
		// should produce an InvalidArgument error. This test is meant
		// to validate how that error is handled (its propagated back
		// to the caller)
		ref := resourceID(
			&pbresource.Type{Group: "not", GroupVersion: "v1", Kind: "found"},
			"irrelevant",
			tenancy,
		)
		health, err := getNodeHealth(context.Background(), suite.runtime, ref)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
		require.Error(suite.T(), err)
		require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthNoNode() {
	suite.runTestCaseWithTenancies("TestGetNodeHealthNoNode", func(tenancy *pbresource.Tenancy) {
		// This test is meant to ensure that when the node doesn't exist
		// no error is returned but also no data is. The default passing
		// status should then be returned in the same manner as the node
		// existing but with no associated HealthStatus resources.
		ref := resourceID(pbcatalog.NodeType, "foo", tenancy)
		ref.Uid = ulid.Make().String()
		health, err := getNodeHealth(context.Background(), suite.runtime, ref)

		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthNoStatus() {
	suite.runTestCaseWithTenancies("TestGetNodeHealthNoStatus", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodeNoHealth[tenancyString])
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthPassingStatus() {
	suite.runTestCaseWithTenancies("TestGetNodeHealthPassingStatus", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodePassing[tenancyString])
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthCriticalStatus() {
	suite.runTestCaseWithTenancies("TestGetNodeHealthCriticalStatus", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodeCritical[tenancyString])
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthWarningStatus() {
	suite.runTestCaseWithTenancies("TestGetNodeHealthWarningStatus", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodeWarning[tenancyString])
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_WARNING, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestGetNodeHealthMaintenanceStatus() {
	suite.runTestCaseWithTenancies("TestGetNodeHealthMaintenanceStatus", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		health, err := getNodeHealth(context.Background(), suite.runtime, suite.nodeMaintenance[tenancyString])
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_MAINTENANCE, health)
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcileNodeNotFound() {
	suite.runTestCaseWithTenancies("TestReconcileNodeNotFound", func(tenancy *pbresource.Tenancy) {
		// This test ensures that removed nodes are ignored. In particular we don't
		// want to propagate the error and indefinitely keep re-reconciling in this case.
		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: resourceID(pbcatalog.NodeType, "not-found", tenancy),
		})
		require.NoError(suite.T(), err)
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcilePropagateReadError() {
	suite.runTestCaseWithTenancies("TestReconcilePropagateReadError", func(tenancy *pbresource.Tenancy) {
		// This test aims to ensure that errors other than NotFound errors coming
		// from the initial resource read get propagated. This case is very unrealistic
		// as the controller should not have given us a request ID for a resource type
		// that doesn't exist but this was the easiest way I could think of to synthesize
		// a Read error.
		ref := resourceID(
			&pbresource.Type{Group: "not", GroupVersion: "v1", Kind: "found"},
			"irrelevant",
			tenancy,
		)

		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: ref,
		})
		require.Error(suite.T(), err)
		require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
	})
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
	suite.runTestCaseWithTenancies("TestReconcile_StatusPassing", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		suite.testReconcileStatus(suite.nodePassing[tenancyString], &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_TRUE,
			Reason:  "HEALTH_PASSING",
			Message: NodeHealthyMessage,
		})
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusWarning() {
	suite.runTestCaseWithTenancies("TestReconcile_StatusPassing", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		suite.testReconcileStatus(suite.nodeWarning[tenancyString], &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_FALSE,
			Reason:  "HEALTH_WARNING",
			Message: NodeUnhealthyMessage,
		})
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusCritical() {
	suite.runTestCaseWithTenancies("TestReconcile_StatusCritical", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		suite.testReconcileStatus(suite.nodeCritical[tenancyString], &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_FALSE,
			Reason:  "HEALTH_CRITICAL",
			Message: NodeUnhealthyMessage,
		})
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_StatusMaintenance() {
	suite.runTestCaseWithTenancies("TestReconcile_StatusMaintenance", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		suite.testReconcileStatus(suite.nodeMaintenance[tenancyString], &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_FALSE,
			Reason:  "HEALTH_MAINTENANCE",
			Message: NodeUnhealthyMessage,
		})
	})
}

func (suite *nodeHealthControllerTestSuite) TestReconcile_AvoidRereconciliationWrite() {
	suite.runTestCaseWithTenancies("TestReconcile_AvoidRereconciliationWrite", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
		res1 := suite.testReconcileStatus(suite.nodeWarning[tenancyString], &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_FALSE,
			Reason:  "HEALTH_WARNING",
			Message: NodeUnhealthyMessage,
		})

		res2 := suite.testReconcileStatus(suite.nodeWarning[tenancyString], &pbresource.Condition{
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
	suite.runTestCaseWithTenancies("TestController", func(tenancy *pbresource.Tenancy) {
		tenancyString := resourcetest.ToTenancyString(tenancy)
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
		suite.waitForReconciliation(suite.nodePassing[tenancyString], "HEALTH_PASSING")

		// rewrite the resource - this will cause the nodes health
		// to be rereconciled but wont result in any health change
		resourcetest.Resource(pbcatalog.NodeType, suite.nodePassing[tenancyString].Name).
			WithData(suite.T(), &pbcatalog.Node{
				Addresses: []*pbcatalog.NodeAddress{
					{
						Host: "198.18.0.1",
					},
				},
			}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.resourceClient)

		// wait for rereconciliation to happen
		suite.waitForReconciliation(suite.nodePassing[tenancyString], "HEALTH_PASSING")

		resourcetest.Resource(pbcatalog.HealthStatusType, "failure").
			WithData(suite.T(), &pbcatalog.HealthStatus{Type: "fake", Status: pbcatalog.Health_HEALTH_CRITICAL}).
			WithOwner(suite.nodePassing[tenancyString]).
			WithTenancy(tenancy).
			Write(suite.T(), suite.resourceClient)

		suite.waitForReconciliation(suite.nodePassing[tenancyString], "HEALTH_CRITICAL")
	})
}

func TestNodeHealthController(t *testing.T) {
	suite.Run(t, new(nodeHealthControllerTestSuite))
}

func (suite *nodeHealthControllerTestSuite) constructTestCaseName(name string, tenancy *pbresource.Tenancy) string {
	if !suite.isEnterprise {
		return name
	}
	return fmt.Sprintf("%s_%s_Namespace_%s_Partition", name, tenancy.Namespace, tenancy.Partition)
}

func (suite *nodeHealthControllerTestSuite) setupNodesWithTenancy(tenancies []*pbresource.Tenancy) {

	// The rest of the setup will be to prime the resource service with some data
	suite.nodeNoHealth = make(map[string]*pbresource.ID)
	suite.nodePassing = make(map[string]*pbresource.ID)
	suite.nodeWarning = make(map[string]*pbresource.ID)
	suite.nodeCritical = make(map[string]*pbresource.ID)
	suite.nodeMaintenance = make(map[string]*pbresource.ID)

	for _, tenancy := range tenancies {

		tenancyString := resourcetest.ToTenancyString(tenancy)

		suite.nodeNoHealth[tenancyString] = suite.writeNode("test-node-no-health", tenancy)
		suite.nodePassing[tenancyString] = suite.writeNode("test-node-passing", tenancy)
		suite.nodeWarning[tenancyString] = suite.writeNode("test-node-warning", tenancy)
		suite.nodeCritical[tenancyString] = suite.writeNode("test-node-critical", tenancy)
		suite.nodeMaintenance[tenancyString] = suite.writeNode("test-node-maintenance", tenancy)

		nodeHealthDesiredStatus := map[string]pbcatalog.Health{
			suite.nodePassing[tenancyString].Name:     pbcatalog.Health_HEALTH_PASSING,
			suite.nodeWarning[tenancyString].Name:     pbcatalog.Health_HEALTH_WARNING,
			suite.nodeCritical[tenancyString].Name:    pbcatalog.Health_HEALTH_CRITICAL,
			suite.nodeMaintenance[tenancyString].Name: pbcatalog.Health_HEALTH_MAINTENANCE,
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

		for _, node := range []*pbresource.ID{suite.nodePassing[tenancyString], suite.nodeWarning[tenancyString], suite.nodeCritical[tenancyString], suite.nodeMaintenance[tenancyString]} {
			for idx, health := range precedenceHealth {
				if nodeHealthDesiredStatus[node.Name] >= health {
					resourcetest.Resource(pbcatalog.HealthStatusType, fmt.Sprintf("test-check-%s-%d-%s-%s", node.Name, idx, tenancy.Partition, tenancy.Namespace)).
						WithData(suite.T(), &pbcatalog.HealthStatus{Type: "tcp", Status: health}).
						WithOwner(node).
						Write(suite.T(), suite.resourceClient)
				}
			}
		}

		// create a DNSPolicy to be owned by the node. The type doesn't really matter it just needs
		// to be something that doesn't care about its owner. All we want to prove is that we are
		// filtering out non-HealthStatus types appropriately.
		resourcetest.Resource(pbcatalog.DNSPolicyType, "test-policy-"+tenancy.Partition+"-"+tenancy.Namespace).
			WithData(suite.T(), dnsPolicyData).
			WithOwner(suite.nodeNoHealth[tenancyString]).
			WithTenancy(tenancy).
			Write(suite.T(), suite.resourceClient)
	}
}

func (suite *nodeHealthControllerTestSuite) runTestCaseWithTenancies(name string, t func(*pbresource.Tenancy)) {
	suite.setupNodesWithTenancy(suite.tenancies)
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.constructTestCaseName(name, tenancy), func() {
			t(tenancy)
		})
	}
}
