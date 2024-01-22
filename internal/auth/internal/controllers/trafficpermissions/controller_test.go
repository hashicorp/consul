// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/controllertest"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/reaper"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/version/versiontest"
)

type controllerSuite struct {
	suite.Suite
	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	ctl          *controller.TestController
	tenancies    []*pbresource.Tenancy
	isEnterprise bool
}

func (suite *controllerSuite) SetupTest() {
	suite.isEnterprise = versiontest.IsEnterprise()
	suite.tenancies = resourcetest.TestTenancies()
	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	suite.ctl = controller.NewTestController(Controller(), client).
		WithLogger(testutil.Logger(suite.T()))
	suite.rt = suite.ctl.Runtime()
	suite.client = rtest.NewClient(suite.rt.Client)
}

func (suite *controllerSuite) requireTrafficPermissionsTracking(tp *pbresource.Resource, expected ...*pbresource.ID) {
	suite.T().Helper()
	iter, err := suite.rt.Cache.ListIterator(pbauth.ComputedTrafficPermissionsType, boundRefsIndexName, tp.Id)
	require.NoError(suite.T(), err)

	var actual []*pbresource.ID
	for res := iter.Next(); res != nil; res = iter.Next() {
		actual = append(actual, res.Id)
	}

	dec := rtest.MustDecode[*pbauth.TrafficPermissions](suite.T(), tp)
	reqs, err := mapTrafficPermissionDestinationIdentity(context.Background(), suite.rt, dec)
	require.NoError(suite.T(), err)
	for _, req := range reqs {
		actual = append(actual, req.ID)
	}

	prototest.AssertElementsMatch(suite.T(), expected, actual)
}

func requireCTP(t testutil.TestingTB, resource *pbresource.Resource, allowExpected []*pbauth.Permission, denyExpected []*pbauth.Permission) {
	t.Helper()
	dec := rtest.MustDecode[*pbauth.ComputedTrafficPermissions](t, resource)
	ctp := dec.Data
	// require.Len(t, ctp.AllowPermissions, len(allowExpected))
	// require.Len(t, ctp.DenyPermissions, len(denyExpected))
	prototest.AssertElementsMatch(t, allowExpected, ctp.AllowPermissions)
	prototest.AssertElementsMatch(t, denyExpected, ctp.DenyPermissions)
}

func (suite *controllerSuite) TestReconcile_CTPCreate_NoReferencingTrafficPermissionsExist() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").WithTenancy(tenancy).Write(suite.T(), suite.client)
		require.NotNil(suite.T(), wi)
		id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(tenancy).WithOwner(wi.Id).ID()
		require.NotNil(suite.T(), id)

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// Ensure that the CTP was created
		ctp := suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctp, []*pbauth.Permission{}, []*pbauth.Permission{})
	})
}

func (suite *controllerSuite) TestReconcile_TPCreate_WorkloadIdentityMissing() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		ctpID := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi1").WithTenancy(tenancy).ID()

		// write the resource so that the cache can be updated with the Traffic Permission.
		tp := rtest.Resource(pbauth.TrafficPermissionsType, "foo").WithTenancy(tenancy).
			WithData(suite.T(), &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "wi1",
				},
				Action: pbauth.Action_ACTION_ALLOW,
			}).
			Write(suite.T(), suite.client)
		require.NotNil(suite.T(), tp)

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: ctpID})
		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), ctpID)
	})
}

func (suite *controllerSuite) TestReconcile_CTPCreate_ReferencingTrafficPermissionsExist() {
	// create dead-end traffic permissions
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		p1 := &pbauth.Permission{
			Sources: []*pbauth.Source{
				{
					IdentityName: "foo",
					Namespace:    "default",
					Partition:    "default",
					Peer:         "local",
				}},
		}
		tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{
				IdentityName: "wi1",
			},
			Action:      pbauth.Action_ACTION_DENY,
			Permissions: []*pbauth.Permission{p1},
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
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
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
		suite.requireTrafficPermissionsTracking(tp2, wi1ID)

		// create the workload identity that they reference
		wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").WithTenancy(tenancy).Write(suite.T(), suite.client)
		id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(tenancy).WithOwner(wi.Id).ID()

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// Ensure that the CTP was created
		ctp := suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
		rtest.RequireOwner(suite.T(), ctp, wi.Id, true)
	})
}

func (suite *controllerSuite) TestReconcile_WorkloadIdentityDelete_ReferencingTrafficPermissionsExist() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		p1 := &pbauth.Permission{
			Sources: []*pbauth.Source{
				{
					IdentityName: "foo",
					Namespace:    "default",
					Partition:    "default",
					Peer:         "local",
				}},
		}
		tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{
				IdentityName: "wi1",
			},
			Action:      pbauth.Action_ACTION_DENY,
			Permissions: []*pbauth.Permission{p1},
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
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
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
		suite.requireTrafficPermissionsTracking(tp2, wi1ID)

		// create the workload identity that they reference
		wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").WithTenancy(tenancy).Write(suite.T(), suite.client)
		id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(tenancy).WithOwner(wi.Id).ID()

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// delete the workload identity
		suite.client.MustDelete(suite.T(), wi.Id)

		// re-reconcile: should untrack the CTP
		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)
	})
}

func (suite *controllerSuite) TestReconcile_WorkloadIdentityDelete_NoReferencingTrafficPermissionsExist() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// create the workload identity that they reference
		wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").WithTenancy(tenancy).Write(suite.T(), suite.client)
		id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(tenancy).WithOwner(wi.Id).ID()

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// delete the workload identity
		suite.client.MustDelete(suite.T(), wi.Id)

		// re-reconcile: should untrack the CTP
		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// there should not be any traffic permissions to compute
		tps, err := suite.rt.Cache.List(pbauth.TrafficPermissionsType, ctpForTPIndexName, id)
		require.NoError(suite.T(), err)
		require.Len(suite.T(), tps, 0)
	})
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsCreate_DestinationWorkloadIdentityExists() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// create the workload identity to be referenced
		wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").WithTenancy(tenancy).Write(suite.T(), suite.client)
		id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(tenancy).WithOwner(wi.Id).ID()

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		ctpResource := suite.client.RequireResourceExists(suite.T(), id)
		assertCTPDefaultStatus(suite.T(), ctpResource, true)

		// create traffic permissions
		p1 := &pbauth.Permission{
			Sources: []*pbauth.Source{
				{
					IdentityName: "foo",
					Namespace:    "default",
					Partition:    "default",
					Peer:         "local",
				}},
		}
		tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithTenancy(tenancy).WithData(suite.T(), &pbauth.TrafficPermissions{
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
		tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithTenancy(tenancy).WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{
				IdentityName: "wi1",
			},
			Action:      pbauth.Action_ACTION_ALLOW,
			Permissions: []*pbauth.Permission{p2},
		}).Write(suite.T(), suite.client)
		suite.requireTrafficPermissionsTracking(tp2, id)

		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// Ensure that the CTP was updated
		ctpResource = suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctpResource, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
		rtest.RequireOwner(suite.T(), ctpResource, wi.Id, true)
		assertCTPDefaultStatus(suite.T(), ctpResource, false)

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
		tp3 := rtest.Resource(pbauth.TrafficPermissionsType, "tp3").WithTenancy(tenancy).WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{
				IdentityName: "wi1",
			},
			Action:      pbauth.Action_ACTION_DENY,
			Permissions: []*pbauth.Permission{p3},
		}).Write(suite.T(), suite.client)
		suite.requireTrafficPermissionsTracking(tp3, id)

		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// Ensure that the CTP was updated
		ctpResource = suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctpResource, []*pbauth.Permission{p2}, []*pbauth.Permission{p1, p3})
		rtest.RequireOwner(suite.T(), ctpResource, wi.Id, true)
		assertCTPDefaultStatus(suite.T(), ctpResource, false)

		// Delete the traffic permissions without updating the caches. Ensure is default is right even when the caches contain stale data.
		suite.client.MustDelete(suite.T(), tp1.Id)
		suite.client.MustDelete(suite.T(), tp2.Id)
		suite.client.MustDelete(suite.T(), tp3.Id)

		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		ctpResource = suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctpResource, []*pbauth.Permission{}, []*pbauth.Permission{})
		rtest.RequireOwner(suite.T(), ctpResource, wi.Id, true)
		assertCTPDefaultStatus(suite.T(), ctpResource, true)
	})
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsCreate_DestinationWorkloadIdentityNotFound() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// create the workload identity to be referenced
		wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").WithTenancy(tenancy).Write(suite.T(), suite.client)
		id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(tenancy).WithOwner(wi.Id).ID()

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		ctpResource := suite.client.RequireResourceExists(suite.T(), id)
		assertCTPDefaultStatus(suite.T(), ctpResource, true)

		// create traffic permissions
		p1 := &pbauth.Permission{
			Sources: []*pbauth.Source{
				{
					IdentityName: "foo",
					Namespace:    "default",
					Partition:    "default",
					Peer:         "local",
				}},
		}
		tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithTenancy(tenancy).WithData(suite.T(), &pbauth.TrafficPermissions{
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
		tp2 := rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithTenancy(tenancy).WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{
				IdentityName: "wi1",
			},
			Action:      pbauth.Action_ACTION_ALLOW,
			Permissions: []*pbauth.Permission{p2},
		}).Write(suite.T(), suite.client)
		suite.requireTrafficPermissionsTracking(tp2, id)

		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// Ensure that the CTP was updated
		ctpResource = suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctpResource, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
		rtest.RequireOwner(suite.T(), ctpResource, wi.Id, true)
		assertCTPDefaultStatus(suite.T(), ctpResource, false)

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
		tp3 := rtest.Resource(pbauth.TrafficPermissionsType, "tp3").WithTenancy(tenancy).WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{
				IdentityName: "wi1",
			},
			Action:      pbauth.Action_ACTION_DENY,
			Permissions: []*pbauth.Permission{p3},
		}).Write(suite.T(), suite.client)
		suite.requireTrafficPermissionsTracking(tp3, id)

		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// Ensure that the CTP was updated
		ctpResource = suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctpResource, []*pbauth.Permission{p2}, []*pbauth.Permission{p1, p3})
		rtest.RequireOwner(suite.T(), ctpResource, wi.Id, true)
		assertCTPDefaultStatus(suite.T(), ctpResource, false)

		// Delete the traffic permissions without updating the caches. Ensure is default is right even when the caches contain stale data.
		suite.client.MustDelete(suite.T(), tp1.Id)
		suite.client.MustDelete(suite.T(), tp2.Id)
		suite.client.MustDelete(suite.T(), tp3.Id)

		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		ctpResource = suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctpResource, []*pbauth.Permission{}, []*pbauth.Permission{})
		rtest.RequireOwner(suite.T(), ctpResource, wi.Id, true)
		assertCTPDefaultStatus(suite.T(), ctpResource, true)
	})
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsDelete_DestinationWorkloadIdentityExists() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// create the workload identity to be referenced
		wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").WithTenancy(tenancy).Write(suite.T(), suite.client)
		id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(tenancy).WithOwner(wi.Id).ID()

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// create traffic permissions
		p1 := &pbauth.Permission{
			Sources: []*pbauth.Source{
				{
					IdentityName: "foo",
					Namespace:    "default",
					Partition:    "default",
					Peer:         "local",
				}},
		}
		tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{
				IdentityName: "wi1",
			},
			Action:      pbauth.Action_ACTION_DENY,
			Permissions: []*pbauth.Permission{p1},
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
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
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
		suite.requireTrafficPermissionsTracking(tp2, id)

		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		ctp := suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1})
		rtest.RequireOwner(suite.T(), ctp, wi.Id, true)

		// Delete TP2
		suite.client.MustDelete(suite.T(), tp2.Id)

		err = suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		// Ensure that the CTP was updated
		ctp = suite.client.RequireResourceExists(suite.T(), id)
		requireCTP(suite.T(), ctp, []*pbauth.Permission{}, []*pbauth.Permission{p1})

		// Ensure TP2 is untracked
		newTps, err := suite.rt.Cache.List(pbauth.TrafficPermissionsType, ctpForTPIndexName, ctp.Id)
		require.NoError(suite.T(), err)
		require.Len(suite.T(), newTps, 1)
		require.Equal(suite.T(), newTps[0].Id.Name, tp1.Id.Name)
	})
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsDelete_DestinationWorkloadIdentityDoesNotExist() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// create traffic permissions
		p1 := &pbauth.Permission{
			Sources: []*pbauth.Source{
				{
					IdentityName: "foo",
					Namespace:    "default",
					Partition:    "default",
					Peer:         "local",
				}},
		}
		tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{
				IdentityName: "wi1",
			},
			Action:      pbauth.Action_ACTION_DENY,
			Permissions: []*pbauth.Permission{p1},
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
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
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
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
	})
}

func (suite *controllerSuite) TestControllerBasic() {
	// TODO: refactor this
	// In this test we check basic operations for a workload identity and referencing traffic permission
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller())
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Add a workload identity
		workloadIdentity := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").WithTenancy(tenancy).Write(suite.T(), suite.client)

		// Wait for the controller to record that the CTP has been computed
		res := suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity.Id), ControllerID)
		// Check that the status was updated
		rtest.RequireStatusCondition(suite.T(), res, ControllerID, ConditionComputed("wi1", true))

		// Check that the CTP resource exists and contains no permissions
		ctpID := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi1").WithTenancy(tenancy).ID()
		ctpObject := suite.client.RequireResourceExists(suite.T(), ctpID)
		requireCTP(suite.T(), ctpObject, nil, nil)

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
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
		suite.client.RequireResourceExists(suite.T(), tp1.Id)
		// Wait for the controller to record that the CTP has been re-computed
		suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity.Id), ControllerID)
		// Check that the ctp has been regenerated
		ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
		rtest.RequireStatusCondition(suite.T(), ctpObject, ControllerID, ConditionComputed("wi1", false))
		// check wi1
		requireCTP(suite.T(), ctpObject, []*pbauth.Permission{p1}, nil)

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
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
		suite.client.RequireResourceExists(suite.T(), tp2.Id)
		// check wi1 is the same
		ctpObject = suite.client.RequireResourceExists(suite.T(), ctpID)
		requireCTP(suite.T(), ctpObject, []*pbauth.Permission{p1}, nil)
		// check no ctp2
		ctpID2 := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi2").WithTenancy(tenancy).ID()
		suite.client.RequireResourceNotFound(suite.T(), ctpID2)

		// delete tp1
		suite.client.MustDelete(suite.T(), tp1.Id)
		suite.client.WaitForDeletion(suite.T(), tp1.Id)
		// check wi1 has no permissions
		ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
		requireCTP(suite.T(), ctpObject, nil, nil)

		// edit tp2 to point to wi1
		rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{IdentityName: "wi1"},
			Action:      pbauth.Action_ACTION_ALLOW,
			Permissions: []*pbauth.Permission{p2},
		}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)
		// check wi1 has tp2's permissions
		ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
		requireCTP(suite.T(), ctpObject, []*pbauth.Permission{p2}, nil)
		// check no ctp2
		ctpID2 = rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi2").WithTenancy(tenancy).ID()
		suite.client.RequireResourceNotFound(suite.T(), ctpID2)
	})
}

// This test tests the behaviour of the Controller when dealing with near identical
// resources present in different tenancies. On a high level here is what the test does
//
//  1. Register two workload identities with the same name in two different tenancies (default+default, foo+bar)
//  2. Register traffic permissions separately in both of those tenants and verify if the CTPs
//     get computed as expected.
//  3. Delete the traffic permission present in the default namespace and partition and verify the
//     changes occurred to the CTP in the same tenant.
//  4. Delete the traffic permission present in the custom namespace and partition and verify the
//     changes occurred to the CTP in the same tenant.
//  5. Add back the traffic permission tp2 to the default namespace and partition and verify the
//     computed CTP in the same tenant
func (suite *controllerSuite) TestControllerBasicWithMultipleTenancyLevels() {
	if !suite.isEnterprise {
		suite.T().Skip("this test should only run against the enterprise build")
	}

	// TODO: refactor this
	// In this test we check basic operations for a workload identity and referencing traffic permission
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller())
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	customTenancy := &pbresource.Tenancy{Partition: "foo", Namespace: "bar"}

	// Add a workload identity in a default namespace and partition
	workloadIdentity1 := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Write(suite.T(), suite.client)

	// Wait for the controller to record that the CTP has been computed
	res := suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity1.Id), ControllerID)
	// Check that the status was updated
	rtest.RequireStatusCondition(suite.T(), res, ControllerID, ConditionComputed("wi1", true))

	// Check that the CTP resource exists and contains no permissions
	ctpID1 := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi1").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	ctpObject1 := suite.client.RequireResourceExists(suite.T(), ctpID1)
	requireCTP(suite.T(), ctpObject1, nil, nil)

	// Add a workload identity with the same name in a custom namespace and partition
	workloadIdentity2 := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").
		WithTenancy(customTenancy).
		Write(suite.T(), suite.client)

	// Wait for the controller to record that the CTP has been computed
	res = suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity2.Id), ControllerID)
	// Check that the status was updated
	rtest.RequireStatusCondition(suite.T(), res, ControllerID, ConditionComputed("wi1", true))

	// Check that the CTP resource exists and contains no permissions
	ctpID2 := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi1").WithTenancy(customTenancy).ID()
	ctpObject2 := suite.client.RequireResourceExists(suite.T(), ctpID2)
	requireCTP(suite.T(), ctpObject2, nil, nil)

	// add a traffic permission that references wi1 present in the default namespace and partition
	p1 := &pbauth.Permission{
		Sources: []*pbauth.Source{{
			IdentityName: "wi2",
			Namespace:    "bar",
			Partition:    "foo",
			Peer:         "local",
		}},
		DestinationRules: nil,
	}
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p1},
	}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp1.Id)
	// Wait for the controller to record that the CTP has been re-computed
	suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity1.Id), ControllerID)
	// Check that the ctp has been regenerated
	ctpObject1 = suite.client.WaitForNewVersion(suite.T(), ctpID1, ctpObject1.Version)
	rtest.RequireStatusCondition(suite.T(), ctpObject1, ControllerID, ConditionComputed("wi1", false))
	// check wi1
	requireCTP(suite.T(), ctpObject1, []*pbauth.Permission{p1}, nil)

	// add a traffic permission that references wi1 present in the custom namespace and partition
	p2 := &pbauth.Permission{
		Sources: []*pbauth.Source{{
			IdentityName: "wi2",
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
	}).
		WithTenancy(customTenancy).
		Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp2.Id)
	// Wait for the controller to record that the CTP has been re-computed
	suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity2.Id), ControllerID)
	// Check that the ctp has been regenerated
	ctpObject2 = suite.client.WaitForNewVersion(suite.T(), ctpID2, ctpObject2.Version)
	rtest.RequireStatusCondition(suite.T(), ctpObject2, ControllerID, ConditionComputed("wi1", false))
	// check wi1
	requireCTP(suite.T(), ctpObject2, []*pbauth.Permission{p2}, nil)

	// delete tp1
	suite.client.MustDelete(suite.T(), tp1.Id)
	suite.client.WaitForDeletion(suite.T(), tp1.Id)
	// check that the CTP in default tenancy has no permissions
	ctpObject1 = suite.client.WaitForNewVersion(suite.T(), ctpID1, ctpObject1.Version)
	requireCTP(suite.T(), ctpObject1, nil, nil)

	// delete tp2 in the custom partition and namespace
	suite.client.MustDelete(suite.T(), tp2.Id)
	suite.client.WaitForDeletion(suite.T(), tp2.Id)
	// check that the CTP in custom tenancy has no permissions
	ctpObject2 = suite.client.WaitForNewVersion(suite.T(), ctpID2, ctpObject2.Version)
	requireCTP(suite.T(), ctpObject2, nil, nil)

	// Add tp2 to point to wi1 in the default partition and namespace
	tp2 = rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Write(suite.T(), suite.client)
	suite.client.RequireResourceExists(suite.T(), tp2.Id)
	// check wi1 in the default partition and namespace has tp2's permissions
	ctpObject1 = suite.client.WaitForNewVersion(suite.T(), ctpID1, ctpObject1.Version)
	requireCTP(suite.T(), ctpObject1, []*pbauth.Permission{p2}, nil)
}

func (suite *controllerSuite) runTestCaseWithTenancies(testFunc func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			testFunc(tenancy)
		})
	}
}

func (suite *controllerSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func assertCTPDefaultStatus(t *testing.T, resource *pbresource.Resource, isDefault bool) {
	dec := rtest.MustDecode[*pbauth.ComputedTrafficPermissions](t, resource)
	require.Equal(t, isDefault, dec.Data.IsDefault)
}

func TestControllerIntegration(t *testing.T) {
	// This test aims to be a very light weight integration test of the controller
	// being executed and managed by the controller manager as well as a general
	// controller lifecycle test.

	client := rtest.NewClient(controllertest.NewControllerTestBuilder().
		WithTenancies(rtest.TestTenancies()...).
		WithResourceRegisterFns(types.Register).
		WithControllerRegisterFns(func(mgr *controller.Manager) {
			mgr.Register(Controller())
		}, reaper.RegisterControllers,
		).
		Run(t))

	for _, tenancy := range rtest.TestTenancies() {
		t.Run(rtest.TenancySubTestName(tenancy), func(t *testing.T) {
			tenancy := tenancy

			// Constant Data to be used at various points throughout the rest of the test
			var (
				p1 = &pbauth.Permission{
					Sources: []*pbauth.Source{{
						IdentityName: "wi2",
						Namespace:    tenancy.Namespace,
						Partition:    tenancy.Partition,
						Peer:         "local",
					}},
					DestinationRules: nil,
				}
				p2 = &pbauth.Permission{
					Sources: []*pbauth.Source{{
						IdentityName: "wi3",
						Namespace:    tenancy.Namespace,
						Partition:    tenancy.Partition,
						Peer:         "local",
					}},
					DestinationRules: nil,
				}
				p3 = &pbauth.Permission{
					Sources: []*pbauth.Source{{
						IdentityName: "wi4",
						Namespace:    tenancy.Namespace,
						Partition:    tenancy.Partition,
						Peer:         "local",
					}},
					DestinationRules: nil,
				}

				wi1ID = &pbresource.ID{
					Name:    "wi1",
					Type:    pbauth.WorkloadIdentityType,
					Tenancy: tenancy,
				}

				wi1Dest = &pbauth.Destination{
					IdentityName: "wi1",
				}

				wi2ID = &pbresource.ID{
					Name:    "wi2",
					Type:    pbauth.WorkloadIdentityType,
					Tenancy: tenancy,
				}

				wi2Dest = &pbauth.Destination{
					IdentityName: "wi2",
				}

				tp1ID = &pbresource.ID{
					Name:    "tp1",
					Type:    pbauth.TrafficPermissionsType,
					Tenancy: tenancy,
				}

				tp2ID = &pbresource.ID{
					Name:    "tp2",
					Type:    pbauth.TrafficPermissionsType,
					Tenancy: tenancy,
				}

				tp3ID = &pbresource.ID{
					Name:    "tp3",
					Type:    pbauth.TrafficPermissionsType,
					Tenancy: tenancy,
				}

				ctp1ID = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi1ID)

				ctp2ID = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi2ID)
			)

			// Create a workload identity and some initial traffic permissions and verify that the
			// controller generates the ComputedTrafficPermissions correctly
			testutil.RunStep(t, "creation", func(t *testing.T) {
				_ = rtest.ResourceID(tp1ID).
					WithData(t, &pbauth.TrafficPermissions{
						Destination: wi1Dest,
						Action:      pbauth.Action_ACTION_ALLOW,
						Permissions: []*pbauth.Permission{p1},
					}).
					WithoutCleanup().
					Write(t, client)
				_ = rtest.ResourceID(tp2ID).
					WithData(t, &pbauth.TrafficPermissions{
						Destination: wi1Dest,
						Action:      pbauth.Action_ACTION_ALLOW,
						Permissions: []*pbauth.Permission{p2},
					}).
					WithoutCleanup().
					Write(t, client)
				_ = rtest.ResourceID(wi1ID).
					WithoutCleanup().
					Write(t, client)

				// Wait for the controller to compute/store the ComputedTrafficPermissions for wi1
				waitForCTP(t, client, ctp1ID, false, []*pbauth.Permission{p1, p2}, nil)
			})

			// Add more traffic permissions for an existing identity and verify that the controller
			// recomputes the ComputedTrafficPermissions correctly
			testutil.RunStep(t, "update", func(t *testing.T) {
				_ = rtest.ResourceID(tp3ID).
					WithData(t, &pbauth.TrafficPermissions{
						Destination: wi1Dest,
						Action:      pbauth.Action_ACTION_DENY,
						Permissions: []*pbauth.Permission{p3},
					}).
					WithoutCleanup().
					Write(t, client)

				// Wait for reconciliation to get things into the correct state
				waitForCTP(t, client, ctp1ID, false, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})
			})

			// Delete a ComputedTrafficPermissions and verify that the controller recreates it correctly
			testutil.RunStep(t, "delete-computed", func(t *testing.T) {
				client.MustDelete(t, ctp1ID)

				// Wait for the controller to recomputed the deleted resource
				waitForCTP(t, client, ctp1ID, false, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})
			})

			// Delete a workload identity and verify that the controller deletes the corresponding
			// ComputedTrafficPermissions resource as expected.
			testutil.RunStep(t, "delete-identity", func(t *testing.T) {
				client.MustDelete(t, wi1ID)

				// Wait for the controller to delete the ComputedTrafficPermissions in response to identity deletion.
				client.WaitForDeletion(t, ctp1ID)
			})

			// Add the deleted identity back and verify that its ComputeTrafficPermissions are recreated
			testutil.RunStep(t, "recreate-identity", func(t *testing.T) {
				_ = rtest.ResourceID(wi1ID).
					WithoutCleanup().
					Write(t, client)

				// Wait for reconciliation to get things into the correct state now that the workload identity is back.
				waitForCTP(t, client, ctp1ID, false, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})
			})

			// Delete a TrafficPermissions and verify that the corresponding ComputedTrafficPermissions
			// is updated to reflect the change in permissions.
			testutil.RunStep(t, "delete-traffic-permission", func(t *testing.T) {
				client.MustDelete(t, tp3ID)

				// Wait for reconciliation to get things into the correct state after removal of tp3
				waitForCTP(t, client, ctp1ID, false, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})
			})

			// Add a new WorkloadIdentity with no TrafficPermissions and verify that the controller
			// creates an empty ComputedTrafficPermissions resource for it.
			testutil.RunStep(t, "create-wi2-identity", func(t *testing.T) {
				_ = rtest.ResourceID(wi2ID).
					WithoutCleanup().
					Write(t, client)

				// Wait for the controller to compute the ComputedTrafficPermissions for wi2
				waitForCTP(t, client, ctp2ID, true, nil, nil)
			})

			// Update the Destination.IdentityName field of some TrafficPermissions and verify
			// that both their current destinations ComputedTrafficPermissions are updated as
			// to add the new permissions and their old destinations ComputedTrafficPermissions
			// are updated to remove those permissions.
			testutil.RunStep(t, "update-traffic-permission-destination", func(t *testing.T) {
				_ = rtest.ResourceID(tp1ID).
					WithData(t, &pbauth.TrafficPermissions{
						Destination: wi2Dest,
						Action:      pbauth.Action_ACTION_ALLOW,
						Permissions: []*pbauth.Permission{p1},
					}).
					WithoutCleanup().
					Write(t, client)
				_ = rtest.ResourceID(tp2ID).
					WithData(t, &pbauth.TrafficPermissions{
						Destination: wi2Dest,
						Action:      pbauth.Action_ACTION_ALLOW,
						Permissions: []*pbauth.Permission{p2},
					}).
					WithoutCleanup().
					Write(t, client)
				_ = rtest.ResourceID(tp3ID).
					WithData(t, &pbauth.TrafficPermissions{
						Destination: wi2Dest,
						Action:      pbauth.Action_ACTION_DENY,
						Permissions: []*pbauth.Permission{p3},
					}).
					WithoutCleanup().
					Write(t, client)

				// Wait for the controller to compute the ComputedTrafficPermissions for wi2
				// Wait for reconciliation to get things into the correct state
				waitForCTP(t, client, ctp2ID, false, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3})

				// Wait for the controller to recompute the ComputedTrafficPermissions for wi1
				// Wait for reconciliation to get things into the correct state
				waitForCTP(t, client, ctp1ID, true, nil, nil)
			})
		})
	}
}

func waitForCTP(t testutil.TestingTB, client *rtest.Client, ctpID *pbresource.ID, isDefault bool, allow []*pbauth.Permission, deny []*pbauth.Permission) {
	t.Helper()
	client.WaitForResourceState(t, ctpID, func(t rtest.T, res *pbresource.Resource) {
		// Quick check to verify that the status condition is set for the current gen
		rtest.RequireStatusConditionForCurrentGen(t, res, ControllerID, ConditionComputed(ctpID.Name, isDefault))

		// Now verify that the resource has the expected permissions
		requireCTP(t, res, allow, deny)
	})
}
