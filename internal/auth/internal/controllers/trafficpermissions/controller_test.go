// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/auth/internal/mappers/trafficpermissionsmapper"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

type controllerSuite struct {
	suite.Suite
	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	mapper     *trafficpermissionsmapper.TrafficPermissionsMapper
	reconciler *reconciler
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.RunResourceService(suite.T(), types.Register)
	suite.client = rtest.NewClient(client)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.mapper = trafficpermissionsmapper.New()
	suite.reconciler = &reconciler{mapper: suite.mapper}
}

func (suite *controllerSuite) requireTrafficPermissionsTracking(tp *pbresource.Resource, ids ...*pbresource.ID) {
	reqs, err := suite.mapper.MapTrafficPermissions(suite.ctx, suite.rt, tp)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), reqs, len(ids))
	for _, id := range ids {
		prototest.AssertContainsElement(suite.T(), reqs, controller.Request{ID: id})
	}
	for _, req := range reqs {
		prototest.AssertContainsElement(suite.T(), ids, req.ID)
	}
}

func (suite *controllerSuite) requireCTP(resource *pbresource.Resource, allowExpected []*pbauth.Permission, denyExpected []*pbauth.Permission) {
	var ctp pbauth.ComputedTrafficPermissions
	require.NoError(suite.T(), resource.Data.UnmarshalTo(&ctp))
	require.Len(suite.T(), ctp.AllowPermissions, len(allowExpected))
	require.Len(suite.T(), ctp.DenyPermissions, len(denyExpected))
	prototest.AssertElementsMatch(suite.T(), allowExpected, ctp.AllowPermissions)
	prototest.AssertElementsMatch(suite.T(), denyExpected, ctp.DenyPermissions)
}

func (suite *controllerSuite) TestReconcile_CTPCreate_NoReferencingTrafficPermissionsExist() {
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	require.NotNil(suite.T(), wi)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()
	require.NotNil(suite.T(), id)

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was created
	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{}, []*pbauth.Permission{})
}

func (suite *controllerSuite) TestReconcile_CTPCreate_ReferencingTrafficPermissionsExist() {
	// create dead-end traffic permissions
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	wi1ID := &pbresource.ID{
		Name:    "wi1",
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp1.Id.Tenancy,
	}
	suite.requireTrafficPermissionsTracking(tp1, wi1ID)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			{
				IdentityName: "wi2",
				Namespace:    "default",
				Partition:    "default",
				Peer:         "local",
			}},
	}
	tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2, wi1ID)

	// create the workload identity that they reference
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was created
	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
	rtest.RequireOwner(suite.T(), ctp, wi.Id, true)
}

func (suite *controllerSuite) TestReconcile_WorkloadIdentityDelete_ReferencingTrafficPermissionsExist() {
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	wi1ID := &pbresource.ID{
		Name:    "wi1",
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp1.Id.Tenancy,
	}
	suite.requireTrafficPermissionsTracking(tp1, wi1ID)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			{
				IdentityName: "wi2",
				Namespace:    "default",
				Partition:    "default",
				Peer:         "local",
			}},
	}
	tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2, wi1ID)

	// create the workload identity that they reference
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// delete the workload identity
	suite.client.MustDelete(suite.T(), wi.Id)

	// re-reconcile: should untrack the CTP
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)
}

func (suite *controllerSuite) TestReconcile_WorkloadIdentityDelete_NoReferencingTrafficPermissionsExist() {
	// create the workload identity that they reference
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// delete the workload identity
	suite.client.MustDelete(suite.T(), wi.Id)

	// re-reconcile: should untrack the CTP
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// there should not be any traffic permissions to compute
	tps := suite.mapper.GetTrafficPermissionsForCTP(id)
	require.Len(suite.T(), tps, 0)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsCreate_DestinationWorkloadIdentityExists() {
	// create the workload identity to be referenced
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// create traffic permissions
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp1, id)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			{
				IdentityName: "wi2",
				Namespace:    "default",
				Partition:    "default",
				Peer:         "local",
			}},
	}
	tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2, id)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was updated
	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
	rtest.RequireOwner(suite.T(), ctp, wi.Id, true)

	// Add another TP
	p3 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			{
				IdentityName: "wi3",
				Namespace:    "default",
				Partition:    "default",
				Peer:         "local",
			}},
	}
	tp3 := rtest.Resource(pbauth.TrafficPermissionsType, "tp3").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p3},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp3, id)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was updated
	ctp = suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1, p3})
	rtest.RequireOwner(suite.T(), ctp, wi.Id, true)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsDelete_DestinationWorkloadIdentityExists() {
	// create the workload identity to be referenced
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// create traffic permissions
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp1, id)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			{
				IdentityName: "wi2",
				Namespace:    "default",
				Partition:    "default",
				Peer:         "local",
			}},
	}
	tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2, id)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
	rtest.RequireOwner(suite.T(), ctp, wi.Id, true)

	// Delete TP2
	suite.client.MustDelete(suite.T(), tp2.Id)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was updated
	ctp = suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{}, []*pbauth.Permission{p1})

	// Ensure TP2 is untracked
	newTps := suite.mapper.GetTrafficPermissionsForCTP(ctp.Id)
	require.Len(suite.T(), newTps, 1)
	require.Equal(suite.T(), newTps[0].Name, tp1.Id.Name)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsDelete_DestinationWorkloadIdentityDoesNotExist() {
	// create traffic permissions
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	wi1ID := &pbresource.ID{
		Name:    "wi1",
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp1.Id.Tenancy,
	}
	suite.requireTrafficPermissionsTracking(tp1, wi1ID)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			{
				IdentityName: "wi2",
				Namespace:    "default",
				Partition:    "default",
				Peer:         "local",
			}},
	}
	tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2, wi1ID)

	// Delete TP2
	suite.client.MustDelete(suite.T(), tp2.Id)

	// Ensure that no CTPs exist
	rsp, err := suite.client.List(suite.ctx, &pbresource.ListRequest{
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	})
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), rsp.Resources)
}

func (suite *controllerSuite) TestControllerBasic() {
	// TODO: refactor this
	// In this test we check basic operations for a workload identity and referencing traffic permission
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller(suite.mapper))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	// Add a workload identity
	workloadIdentity := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)

	// Wait for the controller to record that the CTP has been computed
	res := suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity.Id), StatusKey)
	// Check that the status was updated
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1"))

	// Check that the CTP resource exists and contains no permissions
	ctpID := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi1").ID()
	ctpObject := suite.client.RequireResourceExists(suite.T(), ctpID)
	suite.requireCTP(ctpObject, nil, nil)

	// add a traffic permission that references wi1
	p1 := &pbauth.Permission{
		Sources: []*pbauth.Source{{
			IdentityName: "wi2",
			Namespace:    "default",
			Partition:    "default",
			Peer:         "local",
		}},
		DestinationRules: nil,
	}
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp1.Id)
	// Wait for the controller to record that the CTP has been re-computed
	res = suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity.Id), StatusKey)
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1"))
	// Check that the ctp has been regenerated
	ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
	// check wi1
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1}, nil)

	// add a traffic permission that references wi2
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{{
			IdentityName: "wi1",
			Namespace:    "default",
			Partition:    "default",
			Peer:         "local",
		}},
		DestinationRules: nil,
	}
	tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi2"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp2.Id)
	// check wi1 is the same
	ctpObject = suite.client.RequireResourceExists(suite.T(), ctpID)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1}, nil)
	// check no ctp2
	ctpID2 := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi2").ID()
	suite.client.RequireResourceNotFound(suite.T(), ctpID2)

	// delete tp1
	suite.client.MustDelete(suite.T(), tp1.Id)
	suite.client.WaitForDeletion(suite.T(), tp1.Id)
	// check wi1 has no permissions
	ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
	suite.requireCTP(ctpObject, nil, nil)

	// edit tp2 to point to wi1
	rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	// check wi1 has tp2's permissions
	ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p2}, nil)
	// check no ctp2
	ctpID2 = rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi2").ID()
	suite.client.RequireResourceNotFound(suite.T(), ctpID2)
}

func (suite *controllerSuite) TestControllerMultipleTrafficPermissions() {
	// TODO: refactor this, turn back on once timing flakes are understood
	suite.T().Skip("flaky behavior observed")
	// In this test we check operations for a workload identity and multiple referencing traffic permissions
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller(suite.mapper))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	wi1ID := &pbresource.ID{
		Name:    "wi1",
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	// add tp1 and tp2
	p1 := &pbauth.Permission{
		Sources: []*pbauth.Source{{
			IdentityName: "wi2",
			Namespace:    "default",
			Partition:    "default",
			Peer:         "local",
		}},
		DestinationRules: nil,
	}
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp1.Id)
	suite.requireTrafficPermissionsTracking(tp1, wi1ID)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{{
			IdentityName: "wi3",
			Namespace:    "default",
			Partition:    "default",
			Peer:         "local",
		}},
		DestinationRules: nil,
	}
	tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp2.Id)
	suite.requireTrafficPermissionsTracking(tp1, wi1ID)

	// Add a workload identity
	workloadIdentity := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	ctpID := resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity.Id)
	// Wait for the controller to record that the CTP has been computed
	res := suite.client.WaitForReconciliation(suite.T(), ctpID, StatusKey)
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1"))
	// check ctp1 has tp1 and tp2
	ctpObject := suite.client.RequireResourceExists(suite.T(), res.Id)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, nil)

	// add tp3
	p3 := &pbauth.Permission{
		Sources: []*pbauth.Source{{
			IdentityName: "wi4",
			Namespace:    "default",
			Partition:    "default",
			Peer:         "local",
		}},
		DestinationRules: nil,
	}
	tp3 := rtest.Resource(pbauth.TrafficPermissionsType, "tp3").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p3},
	}).Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp3.Id)
	// check ctp1 has tp3
	ctpObject = suite.client.WaitForReconciliation(suite.T(), ctpObject.Id, StatusKey)
	ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpObject.Id, ctpObject.Version)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})

	// delete ctp
	suite.client.MustDelete(suite.T(), ctpObject.Id)
	suite.client.WaitForDeletion(suite.T(), ctpObject.Id)
	// check ctp regenerated, has all permissions
	res = suite.client.WaitForReconciliation(suite.T(), ctpID, StatusKey)
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1"))
	ctpObject = suite.client.RequireResourceExists(suite.T(), res.Id)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})

	// delete wi1
	suite.client.MustDelete(suite.T(), workloadIdentity.Id)
	suite.client.WaitForDeletion(suite.T(), workloadIdentity.Id)

	// recreate wi1
	rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	// check ctp regenerated, has all permissions
	res = suite.client.WaitForReconciliation(suite.T(), ctpID, StatusKey)
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1"))
	ctpObject = suite.client.RequireResourceExists(suite.T(), res.Id)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})

	// delete tp3
	suite.client.MustDelete(suite.T(), tp3.Id)
	suite.client.WaitForDeletion(suite.T(), tp3.Id)
	suite.client.RequireResourceNotFound(suite.T(), tp3.Id)
	// check ctp1 has tp1 and tp2, and not tp3
	res = suite.client.WaitForReconciliation(suite.T(), ctpObject.Id, StatusKey)
	ctpObject = suite.client.WaitForNewVersion(suite.T(), res.Id, ctpObject.Version)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, nil)

	// add wi2
	workloadIdentity2 := rtest.Resource(pbauth.WorkloadIdentityType, "wi2").Write(suite.T(), suite.client)
	// Wait for the controller to record that the CTP has been computed
	res2 := suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity2.Id), StatusKey)
	rtest.RequireStatusCondition(suite.T(), res2, StatusKey, ConditionComputed("wi2"))
	// check ctp2 has no permissions
	ctpObject2 := suite.client.RequireResourceExists(suite.T(), res2.Id)
	suite.requireCTP(ctpObject2, nil, nil)

	// edit all traffic permissions to point to wi2
	tp1 = rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi2"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	tp2 = rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi2"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	tp3 = rtest.Resource(pbauth.TrafficPermissionsType, "tp3").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi2"},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p3},
	}).Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp1.Id)
	suite.client.RequireResourceExists(suite.T(), tp2.Id)
	suite.client.RequireResourceExists(suite.T(), tp3.Id)

	// check wi2 has updated with all permissions after 6 reconciles
	ctpObject2 = suite.client.WaitForReconciliation(suite.T(), ctpObject2.Id, StatusKey)
	res2 = suite.client.WaitForReconciliation(suite.T(), ctpObject2.Id, StatusKey)
	suite.client.WaitForResourceState(suite.T(), res2.Id, func(t rtest.T, res *pbresource.Resource) {
		suite.requireCTP(res, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})
	})
	// check wi1 has no permissions after 6 reconciles
	ctpObject = suite.client.WaitForReconciliation(suite.T(), ctpObject.Id, StatusKey)
	res = suite.client.WaitForReconciliation(suite.T(), ctpObject.Id, StatusKey)
	suite.client.WaitForResourceState(suite.T(), res.Id, func(t rtest.T, res *pbresource.Resource) {
		suite.requireCTP(res, nil, nil)
	})
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}
