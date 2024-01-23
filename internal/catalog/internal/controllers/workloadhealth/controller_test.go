// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadhealth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/controllertest"
	"github.com/hashicorp/consul/internal/resource"
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

	fakeType = &pbresource.Type{
		Group:        "not",
		GroupVersion: "vfake",
		Kind:         "found",
	}
)

func resourceID(rtype *pbresource.Type, name string, tenancy *pbresource.Tenancy) *pbresource.ID {
	defaultTenancy := resource.DefaultNamespacedTenancy()
	if tenancy != nil {
		defaultTenancy = tenancy
	}

	return &pbresource.ID{
		Type:    rtype,
		Tenancy: defaultTenancy,
		Name:    name,
	}
}

func workloadData(nodeName string) *pbcatalog.Workload {
	return &pbcatalog.Workload{
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
		Identity: "test",
		NodeName: nodeName,
	}
}

// controllerSuite is just the base information the three other test suites
// in this file will use. It will be embedded into the others allowing
// for the test helpers and default setup to be reused and to force consistent
// anming of the various data bits this holds on to.
type controllerSuite struct {
	suite.Suite
	client  pbresource.ResourceServiceClient
	runtime controller.Runtime

	isEnterprise bool
	tenancies    []*pbresource.Tenancy
	ctl          *controller.TestController
}

func (suite *controllerSuite) SetupTest() {
	suite.tenancies = resourcetest.TestTenancies()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register).
		WithTenancies(suite.tenancies...).
		Run(suite.T())
	suite.ctl = controller.NewTestController(WorkloadHealthController(), client).
		WithLogger(testutil.Logger(suite.T()))
	suite.runtime = suite.ctl.Runtime()
	suite.client = suite.runtime.Client
	suite.isEnterprise = versiontest.IsEnterprise()
}

// injectNodeWithStatus is a helper method to write a Node resource and synthesize its status
// in a manner consistent with the node-health controller. This allows us to not actually
// run and test the node-health controller but consume its "api" in the form of how
// it encodes status.
func injectNodeWithStatus(t testutil.TestingTB, client pbresource.ResourceServiceClient, name string, health pbcatalog.Health, tenancy *pbresource.Tenancy) *pbresource.Resource {
	t.Helper()
	state := pbresource.Condition_STATE_TRUE
	if health >= pbcatalog.Health_HEALTH_WARNING {
		state = pbresource.Condition_STATE_FALSE
	}

	return resourcetest.Resource(pbcatalog.NodeType, name).
		WithData(t, nodeData).
		WithTenancy(&pbresource.Tenancy{
			Partition: tenancy.Partition,
		}).
		WithStatus(nodehealth.StatusKey, &pbresource.Status{
			Conditions: []*pbresource.Condition{
				{
					Type:   nodehealth.StatusConditionHealthy,
					State:  state,
					Reason: health.String(),
				},
			},
		}).
		Write(t, client)
}

// the workloadHealthControllerTestSuite intends to test the main Reconciliation
// functionality but will not do exhaustive testing of the getNodeHealth
// or getWorkloadHealth functions. Without mocking the resource service which
// we for now are avoiding, it should be impossible to inject errors into
// those functions that would force some kinds of error cases. Therefore,
// those other functions will be tested with their own test suites.
type workloadHealthControllerTestSuite struct {
	controllerSuite
}

func (suite *workloadHealthControllerTestSuite) SetupTest() {
	// invoke all the other suite setup
	suite.controllerSuite.SetupTest()
}

// testReconcileWithNode will inject a node with the given health, a workload
// associated with that node and then a health status owned by the workload
// with the given workload health. Once all the resource injection has been
// performed this will invoke the Reconcile method once on the reconciler
// and checks a couple things:
//
// * The node to workload association is now being tracked by the node mapper
// * The workloads status was updated and now matches the expected value
func (suite *workloadHealthControllerTestSuite) testReconcileWithNode(nodeHealth, workloadHealth pbcatalog.Health, tenancy *pbresource.Tenancy, status *pbresource.Condition) *pbresource.Resource {
	suite.T().Helper()

	node := injectNodeWithStatus(suite.T(), suite.client, "test-node", nodeHealth, tenancy)

	workload := resourcetest.Resource(pbcatalog.WorkloadType, "test-workload").
		WithData(suite.T(), workloadData(node.Id.Name)).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	resourcetest.Resource(pbcatalog.HealthStatusType, "test-status").
		WithData(suite.T(), &pbcatalog.HealthStatus{Type: "tcp", Status: workloadHealth}).
		WithOwner(workload.Id).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	err := suite.ctl.Reconcile(context.Background(), controller.Request{
		ID: workload.Id,
	})

	require.NoError(suite.T(), err)

	return suite.checkWorkloadStatus(workload.Id, status)
}

// testReconcileWithoutNode will inject a workload associated and then a health status
// owned by the workload with the given workload health. Once all the resource injection
// has been performed this will invoke the Reconcile method once on the reconciler
// and check that the computed status matches the expected value
//
// This is really just a tirmmed down version of testReconcileWithNode. It seemed
// simpler and easier to read if these were two separate methods instead of combining
// them in one with more branching based off of detecting whether nodes are in use.
func (suite *workloadHealthControllerTestSuite) testReconcileWithoutNode(workloadHealth pbcatalog.Health, tenancy *pbresource.Tenancy, status *pbresource.Condition) *pbresource.Resource {
	suite.T().Helper()
	workload := resourcetest.Resource(pbcatalog.WorkloadType, "test-workload").
		WithData(suite.T(), workloadData("")).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	resourcetest.Resource(pbcatalog.HealthStatusType, "test-status").
		WithData(suite.T(), &pbcatalog.HealthStatus{Type: "tcp", Status: workloadHealth}).
		WithTenancy(tenancy).
		WithOwner(workload.Id).
		Write(suite.T(), suite.client)

	err := suite.ctl.Reconcile(context.Background(), controller.Request{
		ID: workload.Id,
	})

	require.NoError(suite.T(), err)

	// Read the resource back so we can detect the status changes
	return suite.checkWorkloadStatus(workload.Id, status)
}

// checkWorkloadStatus will read the workload resource and verify that its
// status has the expected value.
func (suite *workloadHealthControllerTestSuite) checkWorkloadStatus(id *pbresource.ID, status *pbresource.Condition) *pbresource.Resource {
	suite.T().Helper()

	rsp, err := suite.client.Read(context.Background(), &pbresource.ReadRequest{
		Id: id,
	})

	require.NoError(suite.T(), err)

	actualStatus, found := rsp.Resource.Status[ControllerID]
	require.True(suite.T(), found)
	require.Equal(suite.T(), rsp.Resource.Generation, actualStatus.ObservedGeneration)
	require.Len(suite.T(), actualStatus.Conditions, 1)
	prototest.AssertDeepEqual(suite.T(), status, actualStatus.Conditions[0])

	return rsp.Resource
}

func (suite *workloadHealthControllerTestSuite) TestReconcile() {
	// This test intends to ensure all the permutations of node health and workload
	// health end up with the correct computed status. When a test case omits
	// the workload health (or sets it to pbcatalog.Health_HEALTH_ANY) then the
	// workloads are nodeless and therefore node health will not be considered.
	// Additionally the messages put in the status for nodeless workloads are
	// a little different to not mention nodes and provide the user more context
	// about where the failing health checks are.

	type testCase struct {
		nodeHealth     pbcatalog.Health
		workloadHealth pbcatalog.Health
		expectedStatus *pbresource.Condition
	}

	cases := map[string]testCase{
		"workload-passing": {
			workloadHealth: pbcatalog.Health_HEALTH_PASSING,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_TRUE,
				Reason:  "HEALTH_PASSING",
				Message: WorkloadHealthyMessage,
			},
		},
		"workload-warning": {
			workloadHealth: pbcatalog.Health_HEALTH_WARNING,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_WARNING",
				Message: WorkloadUnhealthyMessage,
			},
		},
		"workload-critical": {
			workloadHealth: pbcatalog.Health_HEALTH_CRITICAL,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_CRITICAL",
				Message: WorkloadUnhealthyMessage,
			},
		},
		"workload-maintenance": {
			workloadHealth: pbcatalog.Health_HEALTH_MAINTENANCE,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_MAINTENANCE",
				Message: WorkloadUnhealthyMessage,
			},
		},
		"combined-passing": {
			nodeHealth:     pbcatalog.Health_HEALTH_PASSING,
			workloadHealth: pbcatalog.Health_HEALTH_PASSING,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_TRUE,
				Reason:  "HEALTH_PASSING",
				Message: NodeAndWorkloadHealthyMessage,
			},
		},
		"combined-warning-node": {
			nodeHealth:     pbcatalog.Health_HEALTH_WARNING,
			workloadHealth: pbcatalog.Health_HEALTH_PASSING,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_WARNING",
				Message: nodehealth.NodeUnhealthyMessage,
			},
		},
		"combined-warning-workload": {
			nodeHealth:     pbcatalog.Health_HEALTH_PASSING,
			workloadHealth: pbcatalog.Health_HEALTH_WARNING,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_WARNING",
				Message: WorkloadUnhealthyMessage,
			},
		},
		"combined-critical-node": {
			nodeHealth:     pbcatalog.Health_HEALTH_CRITICAL,
			workloadHealth: pbcatalog.Health_HEALTH_WARNING,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_CRITICAL",
				Message: NodeAndWorkloadUnhealthyMessage,
			},
		},
		"combined-critical-workload": {
			nodeHealth:     pbcatalog.Health_HEALTH_WARNING,
			workloadHealth: pbcatalog.Health_HEALTH_CRITICAL,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_CRITICAL",
				Message: NodeAndWorkloadUnhealthyMessage,
			},
		},
		"combined-maintenance-node": {
			nodeHealth:     pbcatalog.Health_HEALTH_MAINTENANCE,
			workloadHealth: pbcatalog.Health_HEALTH_CRITICAL,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_MAINTENANCE",
				Message: NodeAndWorkloadUnhealthyMessage,
			},
		},
		"combined-maintenance-workload": {
			nodeHealth:     pbcatalog.Health_HEALTH_CRITICAL,
			workloadHealth: pbcatalog.Health_HEALTH_MAINTENANCE,
			expectedStatus: &pbresource.Condition{
				Type:    StatusConditionHealthy,
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "HEALTH_MAINTENANCE",
				Message: NodeAndWorkloadUnhealthyMessage,
			},
		},
	}

	for name, tcase := range cases {
		suite.Run(name, func() {
			suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
				if tcase.nodeHealth != pbcatalog.Health_HEALTH_ANY {
					suite.testReconcileWithNode(tcase.nodeHealth, tcase.workloadHealth, tenancy, tcase.expectedStatus)
				} else {
					suite.testReconcileWithoutNode(tcase.workloadHealth, tenancy, tcase.expectedStatus)
				}
			})
		})
	}
}

func (suite *workloadHealthControllerTestSuite) TestReconcileReadError() {
	// This test's goal is to prove that errors other than NotFound from the Resource service
	// when reading the workload to reconcile will be propagate back to the Reconcile caller.
	//
	// Passing a resource with an unknown type isn't particularly realistic as the controller
	// manager running our reconciliation will ensure all resource ids used are valid. However
	// its a really easy way right not to force the error.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		id := resourceID(fakeType, "blah", tenancy)

		err := suite.ctl.Reconcile(context.Background(), controller.Request{ID: id})
		require.Error(suite.T(), err)
		require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
	})
}

func (suite *workloadHealthControllerTestSuite) TestGetNodeHealthError() {
	// This test aims to ensure that errors coming from the getNodeHealth
	// function are propagated back to the caller. In order to do so
	// we are going to inject a node but not set its status yet. This
	// simulates the condition where the workload health controller happened
	// to start reconciliation before the node health controller. In that
	// case we also expect the errNodeUnreconciled error to be returned
	// but the exact error isn't very relevant to the core reason this
	// test exists.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		node := resourcetest.Resource(pbcatalog.NodeType, "test-node").
			WithData(suite.T(), nodeData).
			WithTenancy(&pbresource.Tenancy{
				Partition: tenancy.Partition,
			}).
			Write(suite.T(), suite.client)

		workload := resourcetest.Resource(pbcatalog.WorkloadType, "test-workload").
			WithData(suite.T(), workloadData(node.Id.Name)).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		resourcetest.Resource(pbcatalog.HealthStatusType, "test-status").
			WithData(suite.T(), &pbcatalog.HealthStatus{Type: "tcp", Status: pbcatalog.Health_HEALTH_CRITICAL}).
			WithOwner(workload.Id).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		err := suite.ctl.Reconcile(context.Background(), controller.Request{
			ID: workload.Id,
		})

		require.Error(suite.T(), err)
		require.Equal(suite.T(), errNodeUnreconciled, err)
	})
}

func (suite *workloadHealthControllerTestSuite) TestReconcile_AvoidReconciliationWrite() {
	// The sole purpose of this test is to ensure that calls to Reconcile for an already
	// reconciled workload will not perform extra/unnecessary status writes. Basically
	// we check that calling Reconcile twice in a row without any actual health change
	// doesn't bump the Version (which would increased for any write of the resource
	// or its status)
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		status := &pbresource.Condition{
			Type:    StatusConditionHealthy,
			State:   pbresource.Condition_STATE_FALSE,
			Reason:  "HEALTH_WARNING",
			Message: WorkloadUnhealthyMessage,
		}
		res1 := suite.testReconcileWithoutNode(pbcatalog.Health_HEALTH_WARNING, tenancy, status)

		err := suite.ctl.Reconcile(context.Background(), controller.Request{ID: res1.Id})
		require.NoError(suite.T(), err)

		// check that the status hasn't changed
		res2 := suite.checkWorkloadStatus(res1.Id, status)

		// If another status write was performed then the versions would differ. This
		// therefore proves that after a second reconciliation without any change
		// in status that the controller is not making extra status writes.
		require.Equal(suite.T(), res1.Version, res2.Version)
	})
}

func TestController(t *testing.T) {
	// This test aims to be a very light weight integration test of the
	// controller with the controller manager as well as a general
	// controller lifecycle test.

	// create the controller manager
	client := controllertest.NewControllerTestBuilder().
		WithTenancies(resourcetest.TestTenancies()...).
		WithResourceRegisterFns(types.Register).
		WithControllerRegisterFns(func(mgr *controller.Manager) {
			mgr.Register(WorkloadHealthController())
		}).
		Run(t)

	for _, tenancy := range resourcetest.TestTenancies() {
		t.Run(tenancySubTestName(tenancy), func(t *testing.T) {
			tenancy := tenancy

			node := injectNodeWithStatus(t, client, "test-node", pbcatalog.Health_HEALTH_PASSING, tenancy)

			// create the workload
			workload := resourcetest.Resource(pbcatalog.WorkloadType, "test-workload").
				WithData(t, workloadData(node.Id.Name)).
				WithTenancy(tenancy).
				Write(t, client)

			// Wait for reconciliation to occur and mark the workload as passing.
			waitForReconciliation(t, client, workload.Id, "HEALTH_PASSING")

			// Simulate a node unhealthy
			injectNodeWithStatus(t, client, "test-node", pbcatalog.Health_HEALTH_WARNING, tenancy)

			// Wait for reconciliation to occur and mark the workload as warning
			// due to the node going into the warning state.
			waitForReconciliation(t, client, workload.Id, "HEALTH_WARNING")

			// Now register a critical health check that should supercede the nodes
			// warning status

			resourcetest.Resource(pbcatalog.HealthStatusType, "test-status").
				WithData(t, &pbcatalog.HealthStatus{Type: "tcp", Status: pbcatalog.Health_HEALTH_CRITICAL}).
				WithOwner(workload.Id).
				WithTenancy(tenancy).
				Write(t, client)

			// Wait for reconciliation to occur again and mark the workload as unhealthy
			waitForReconciliation(t, client, workload.Id, "HEALTH_CRITICAL")

			// Put the health status back into a passing state and delink the node
			resourcetest.Resource(pbcatalog.HealthStatusType, "test-status").
				WithData(t, &pbcatalog.HealthStatus{Type: "tcp", Status: pbcatalog.Health_HEALTH_PASSING}).
				WithOwner(workload.Id).
				WithTenancy(tenancy).
				Write(t, client)
			workload = resourcetest.Resource(pbcatalog.WorkloadType, "test-workload").
				WithData(t, workloadData("")).
				WithTenancy(tenancy).
				Write(t, client)

			// Now that the workload health is passing and its not associated with the node its status should
			// eventually become passing
			waitForReconciliation(t, client, workload.Id, "HEALTH_PASSING")
		})
	}
}

// wait for reconciliation is a helper to check if a resource has been reconciled and
// is marked with the expected status.
func waitForReconciliation(t testutil.TestingTB, client pbresource.ResourceServiceClient, id *pbresource.ID, reason string) {
	t.Helper()

	retry.RunWith(&retry.Timer{Wait: 100 * time.Millisecond, Timeout: 5 * time.Second},
		t, func(r *retry.R) {
			rsp, err := client.Read(context.Background(), &pbresource.ReadRequest{
				Id: id,
			})
			require.NoError(r, err)

			status, found := rsp.Resource.Status[ControllerID]
			require.True(r, found)
			require.Equal(r, rsp.Resource.Generation, status.ObservedGeneration)
			require.Len(r, status.Conditions, 1)
			require.Equal(r, reason, status.Conditions[0].Reason)
		})
}

func TestWorkloadHealthController_Reconcile(t *testing.T) {
	suite.Run(t, new(workloadHealthControllerTestSuite))
}

type getWorkloadHealthTestSuite struct {
	controllerSuite
}

func (suite *getWorkloadHealthTestSuite) addHealthStatuses(workload *pbresource.ID, tenancy *pbresource.Tenancy, desiredHealth pbcatalog.Health) {
	// In order to exercise the behavior to ensure that the ordering a health status is
	// seen doesn't matter this is strategically naming health status so that they will be
	// returned in an order with the most precedent status being in the middle of the list.
	// This will ensure that statuses seen later can override a previous status that that
	// status seen later do not override if they would lower the overall status such as
	// going from critical -> warning.
	healthStatuses := []pbcatalog.Health{
		pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_MAINTENANCE,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_PASSING,
	}

	for idx, health := range healthStatuses {
		if desiredHealth >= health {
			resourcetest.Resource(pbcatalog.HealthStatusType, fmt.Sprintf("check-%s-%d", workload.Name, idx)).
				WithData(suite.T(), &pbcatalog.HealthStatus{Type: "tcp", Status: health}).
				WithTenancy(tenancy).
				WithOwner(workload).
				Write(suite.T(), suite.client)
		}
	}
}

func (suite *getWorkloadHealthTestSuite) TestListError() {
	// This test's goal is to exercise the error propgataion behavior within
	// getWorkloadHealth. When the resource listing fails, we want to
	// propagate the error which should eventually result in retrying
	// the operation.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		health, err := getWorkloadHealth(context.Background(), suite.runtime, resourceID(fakeType, "foo", tenancy))

		require.Error(suite.T(), err)
		require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	})
}

func (suite *getWorkloadHealthTestSuite) TestNoHealthStatuses() {
	// This test's goal is to ensure that when no HealthStatuses are owned by the
	// workload that the health is assumed to be passing.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		workload := resourcetest.Resource(pbcatalog.WorkloadType, "foo").
			WithData(suite.T(), workloadData("")).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		health, err := getWorkloadHealth(context.Background(), suite.runtime, workload.Id)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_PASSING, health)
	})
}

func (suite *getWorkloadHealthTestSuite) TestWithStatuses() {
	// This test's goal is to ensure that the health calculation given multiple
	// statuses results in the most precedent winning. The addHealthStatuses
	// helper method is used to inject multiple statuses in a way such that
	// the resource service will return them in a predictable order and can
	// properly exercise the code.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		for value, status := range pbcatalog.Health_name {
			health := pbcatalog.Health(value)
			if health == pbcatalog.Health_HEALTH_ANY {
				continue
			}

			suite.Run(status, func() {
				workload := resourcetest.Resource(pbcatalog.WorkloadType, "foo").
					WithData(suite.T(), workloadData("")).
					WithTenancy(tenancy).
					Write(suite.T(), suite.client)

				suite.addHealthStatuses(workload.Id, tenancy, health)

				actualHealth, err := getWorkloadHealth(context.Background(), suite.runtime, workload.Id)
				require.NoError(suite.T(), err)
				require.Equal(suite.T(), health, actualHealth)
			})
		}
	})
}

func TestGetWorkloadHealth(t *testing.T) {
	suite.Run(t, new(getWorkloadHealthTestSuite))
}

type getNodeHealthTestSuite struct {
	controllerSuite
}

func (suite *getNodeHealthTestSuite) TestNotfound() {
	// This test's goal is to ensure that getNodeHealth when called with a node id that isn't
	// present in the system results in a the critical health but no error. This situation
	// could occur when a linked node gets removed without the workloads being modified/removed.
	// When that occurs we want to steer traffic away from the linked node as soon as possible.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		health, err := getNodeHealth(context.Background(), suite.runtime, resourceID(pbcatalog.NodeType, "not-found", &pbresource.Tenancy{
			Partition: tenancy.Partition,
		}))
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	})
}

func (suite *getNodeHealthTestSuite) TestReadError() {
	// This test's goal is to ensure the getNodeHealth propagates unexpected errors from
	// its resource read call back to the caller.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		health, err := getNodeHealth(context.Background(), suite.runtime, resourceID(fakeType, "not-found", tenancy))
		require.Error(suite.T(), err)
		require.Equal(suite.T(), codes.InvalidArgument, status.Code(err))
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	})
}

func (suite *getNodeHealthTestSuite) TestUnreconciled() {
	// This test's goal is to ensure that nodes with unreconciled health are deemed
	// critical. Basically, the workload health controller should defer calculating
	// the workload health until the associated nodes health is known.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		node := resourcetest.Resource(pbcatalog.NodeType, "unreconciled").
			WithData(suite.T(), nodeData).
			WithTenancy(&pbresource.Tenancy{
				Partition: tenancy.Partition,
			}).
			Write(suite.T(), suite.client).
			GetId()

		health, err := getNodeHealth(context.Background(), suite.runtime, node)
		require.Error(suite.T(), err)
		require.Equal(suite.T(), errNodeUnreconciled, err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	})
}

func (suite *getNodeHealthTestSuite) TestNoConditions() {
	// This test's goal is to ensure that if a node's health status doesn't have
	// the expected condition then its deemed critical. This should never happen
	// in the integrated system as the node health controller would have to be
	// buggy to add an empty status. However it could also indicate some breaking
	// change went in. Regardless, the code to handle this state is written
	// and it will be tested here.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		node := resourcetest.Resource(pbcatalog.NodeType, "no-conditions").
			WithData(suite.T(), nodeData).
			WithTenancy(&pbresource.Tenancy{
				Partition: tenancy.Partition,
			}).
			WithStatus(nodehealth.StatusKey, &pbresource.Status{}).
			Write(suite.T(), suite.client).
			GetId()

		health, err := getNodeHealth(context.Background(), suite.runtime, node)
		require.Error(suite.T(), err)
		require.Equal(suite.T(), errNodeHealthConditionNotFound, err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	})
}

func (suite *getNodeHealthTestSuite) TestInvalidReason() {
	// This test has the same goal as TestNoConditions which is to ensure that if
	// the node health status isn't properly formed then we assume it is unhealthy.
	// Just like that other test, it should be impossible for the normal running
	// system to actually get into this state or at least for the node-health
	// controller to put it into this state. As users or other controllers could
	// potentially force it into this state by writing the status themselves, it
	// would be good to ensure the defined behavior works as expected.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		node := resourcetest.Resource(pbcatalog.NodeType, "invalid-reason").
			WithData(suite.T(), nodeData).
			WithTenancy(&pbresource.Tenancy{
				Partition: tenancy.Partition,
			}).
			WithStatus(nodehealth.StatusKey, &pbresource.Status{
				Conditions: []*pbresource.Condition{
					{
						Type:   nodehealth.StatusConditionHealthy,
						State:  pbresource.Condition_STATE_FALSE,
						Reason: "INVALID_REASON",
					},
				},
			}).
			Write(suite.T(), suite.client).
			GetId()

		health, err := getNodeHealth(context.Background(), suite.runtime, node)
		require.Error(suite.T(), err)
		require.Equal(suite.T(), errNodeHealthInvalid, err)
		require.Equal(suite.T(), pbcatalog.Health_HEALTH_CRITICAL, health)
	})
}

func (suite *getNodeHealthTestSuite) TestValidHealth() {
	// This test aims to ensure that all status that would be reported by the node-health
	// controller gets accurately detected and returned by the getNodeHealth function.
	suite.controllerSuite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		for value, healthStr := range pbcatalog.Health_name {
			health := pbcatalog.Health(value)

			// this is not a valid health that a health status
			// may be in.
			if health == pbcatalog.Health_HEALTH_ANY {
				continue
			}

			suite.T().Run(healthStr, func(t *testing.T) {
				node := injectNodeWithStatus(suite.T(), suite.client, "test-node", health, tenancy)

				actualHealth, err := getNodeHealth(context.Background(), suite.runtime, node.Id)
				require.NoError(t, err)
				require.Equal(t, health, actualHealth)
			})
		}
	})
}

func TestGetNodeHealth(t *testing.T) {
	suite.Run(t, new(getNodeHealthTestSuite))
}

func (suite *controllerSuite) runTestCaseWithTenancies(testFunc func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(tenancySubTestName(tenancy), func() {
			testFunc(tenancy)
		})
	}
}

func tenancySubTestName(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}
