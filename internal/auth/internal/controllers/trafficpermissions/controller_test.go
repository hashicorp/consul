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
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
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
	wi := rtest.Resource(types.WorkloadIdentityType, "wi1").WithTenancy(resource.DefaultNamespacedTenancy()).Write(suite.T(), suite.client)
	require.NotNil(suite.T(), wi)
	id := rtest.Resource(types.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).ID()
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
	tp1 := rtest.Resource(types.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp1)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			&pbauth.Source{
				IdentityName: "wi2",
			}},
	}
	tp2 := rtest.Resource(types.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2)

	// create the workload identity that they reference
	wi := rtest.Resource(types.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(types.ComputedTrafficPermissionsType, wi.Id.Name).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was created
	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
	rtest.RequireOwner(suite.T(), ctp, wi.Id, true)
}

func (suite *controllerSuite) TestReconcile_WorkloadIdentityDelete_ReferencingTrafficPermissionsExist() {
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(types.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp1)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			&pbauth.Source{
				IdentityName: "wi2",
			}},
	}
	tp2 := rtest.Resource(types.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2)

	// create the workload identity that they reference
	wi := rtest.Resource(types.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(types.ComputedTrafficPermissionsType, wi.Id.Name).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// delete the workload identity
	suite.client.MustDelete(suite.T(), wi.Id)

	// re-reconcile: should untrack the CTP
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// traffic permissions should not be mapped to CTP requests
	reqs1, err := suite.mapper.MapTrafficPermissions(suite.ctx, suite.rt, tp1)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), reqs1)
	reqs2, err := suite.mapper.MapTrafficPermissions(suite.ctx, suite.rt, tp2)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), reqs2)

	// traffic permissions should still be mapped to the WI name
	tps := suite.mapper.GetTrafficPermissionsForCTP(id)
	require.NotNil(suite.T(), tps)
	require.Len(suite.T(), tps, 2)
}

func (suite *controllerSuite) TestReconcile_WorkloadIdentityDelete_NoReferencingTrafficPermissionsExist() {
	// create the workload identity that they reference
	wi := rtest.Resource(types.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(types.ComputedTrafficPermissionsType, wi.Id.Name).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// delete the workload identity
	suite.client.MustDelete(suite.T(), wi.Id)

	// re-reconcile: should untrack the CTP
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// traffic permissions should still be mapped to the WI name
	tps := suite.mapper.GetTrafficPermissionsForCTP(id)
	require.NotNil(suite.T(), tps)
	require.Len(suite.T(), tps, 0)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsCreate_DestinationWorkloadIdentityExists() {
	// create the workload identity to be referenced
	wi := rtest.Resource(types.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(types.ComputedTrafficPermissionsType, wi.Id.Name).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// create traffic permissions
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(types.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp1)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			&pbauth.Source{
				IdentityName: "wi2",
			}},
	}
	tp2 := rtest.Resource(types.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was updated
	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
	rtest.RequireOwner(suite.T(), ctp, wi.Id, true)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsDelete_DestinationWorkloadIdentityExists() {
	// create the workload identity to be referenced
	wi := rtest.Resource(types.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(types.ComputedTrafficPermissionsType, wi.Id.Name).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// create traffic permissions
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(types.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp1)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			&pbauth.Source{
				IdentityName: "wi2",
			}},
	}
	tp2 := rtest.Resource(types.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2)

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
	reqs, err := suite.mapper.MapTrafficPermissions(suite.ctx, suite.rt, tp2)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), reqs)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsDelete_DestinationWorkloadIdentityDoesNotExist() {
	// create traffic permissions
	p1 := &pbauth.Permission{}
	tp1 := rtest.Resource(types.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp1)
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{
			&pbauth.Source{
				IdentityName: "wi2",
			}},
	}
	tp2 := rtest.Resource(types.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsTracking(tp2)

	// Delete TP2
	suite.client.MustDelete(suite.T(), tp2.Id)

	// Ensure that no CTPs exist
	rsp, err := suite.client.List(suite.ctx, &pbresource.ListRequest{
		Type:    types.ComputedTrafficPermissionsType,
		Tenancy: tp2.Id.Tenancy,
	})
	require.Empty(suite.T(), rsp.Resources)

	// Ensure TP2 is untracked
	reqs, err := suite.mapper.MapTrafficPermissions(suite.ctx, suite.rt, tp2)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), reqs)
}

func (suite *controllerSuite) TestController() {}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}
