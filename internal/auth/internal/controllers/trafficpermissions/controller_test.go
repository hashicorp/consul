// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/auth/internal/indexers"
	tpm "github.com/hashicorp/consul/internal/auth/internal/mappers/trafficpermissionsmapper"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	cacheindexers "github.com/hashicorp/consul/internal/controller/cache/indexers"
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

	reconciler *reconciler
	cache      cache.Cache
}

func (suite *controllerSuite) SetupTest() {
	suite.cache = cache.New()
	boundRefsIndex := cacheindexers.RefIndex[*pbauth.ComputedTrafficPermissions](
		func(res *resource.DecodedResource[*pbauth.ComputedTrafficPermissions]) []*pbresource.Reference {
			return res.Data.BoundReferences
		},
	)
	suite.cache.AddIndex(pbauth.TrafficPermissionsType, "computed", indexers.TrafficPermissionsIndex())
	suite.cache.AddIndex(pbauth.ComputedTrafficPermissionsType, "bound_references", boundRefsIndex)
	suite.cache.AddType(pbauth.WorkloadIdentityType)

	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.RunResourceService(suite.T(), types.Register)
	suite.client = rtest.NewClient(cache.NewCachedClient(suite.cache, client))
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
		Cache:  suite.cache,
	}

	suite.reconciler = &reconciler{}
}

func (suite *controllerSuite) requireTrafficPermissionsIndex(tp *pbresource.Resource, id *pbresource.ID) {
	suite.rt.Cache.Get(pbauth.TrafficPermissionsType, "computed", id)
}

func (suite *controllerSuite) requireCTP(resource *pbresource.Resource, allowExpected []*pbauth.Permission, denyExpected []*pbauth.Permission, expectedRefs []*pbresource.Reference) {
	dec := rtest.MustDecode[*pbauth.ComputedTrafficPermissions](suite.T(), resource)
	ctp := dec.Data
	require.Len(suite.T(), ctp.AllowPermissions, len(allowExpected))
	require.Len(suite.T(), ctp.DenyPermissions, len(denyExpected))
	prototest.AssertElementsMatch(suite.T(), allowExpected, ctp.AllowPermissions)
	prototest.AssertElementsMatch(suite.T(), denyExpected, ctp.DenyPermissions)
	prototest.AssertElementsMatch(suite.T(), expectedRefs, ctp.BoundReferences)

	for _, ref := range ctp.BoundReferences {
		ctps, err := suite.rt.Cache.List(pbauth.ComputedTrafficPermissionsType, "bound_references", ref)
		require.NoError(suite.T(), err)

		prototest.AssertContainsElement(suite.T(), ctps, resource)
	}
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
	suite.requireCTP(ctp, []*pbauth.Permission{}, []*pbauth.Permission{}, []*pbresource.Reference{})
}

func (suite *controllerSuite) TestReconcile_CTPCreate_ReferencingTrafficPermissionsExist() {
	// create dead-end traffic permissions
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
	}).Write(suite.T(), suite.client)
	wi1ID := &pbresource.ID{
		Name:    "wi1",
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp1.Id.Tenancy,
	}
	suite.requireTrafficPermissionsIndex(tp1, wi1ID)
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
	suite.requireTrafficPermissionsIndex(tp2, wi1ID)

	// create the workload identity that they reference
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was created
	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1}, []*pbresource.Reference{
		resource.ReferenceFromReferenceOrID(tp1.Id),
		resource.ReferenceFromReferenceOrID(tp2.Id),
	})
	rtest.RequireOwner(suite.T(), ctp, wi.Id, true)
}

func (suite *controllerSuite) TestReconcile_WorkloadIdentityDelete_ReferencingTrafficPermissionsExist() {
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
	}).Write(suite.T(), suite.client)
	wi1ID := &pbresource.ID{
		Name:    "wi1",
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp1.Id.Tenancy,
	}
	suite.requireTrafficPermissionsIndex(tp1, wi1ID)
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
	suite.requireTrafficPermissionsIndex(tp2, wi1ID)

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

	// // there should not be any traffic permissions to compute
	// tps := tpm.MapTrafficPermissions.GetTrafficPermissionsForCTP(id)
	// require.Len(suite.T(), tps, 0)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsCreate_DestinationWorkloadIdentityExists() {
	// create the workload identity to be referenced
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
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
	tp1 := rtest.Resource(pbauth.TrafficPermissionsType, "tp1").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_DENY,
		Permissions: []*pbauth.Permission{p1},
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsIndex(tp1, id)
	tp1Ref := resource.ReferenceFromReferenceOrID(tp1.GetId())
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
	suite.requireTrafficPermissionsIndex(tp2, id)
	tp2Ref := resource.ReferenceFromReferenceOrID(tp2.GetId())

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was updated
	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1}, []*pbresource.Reference{
		tp1Ref,
		tp2Ref,
	})
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
	suite.requireTrafficPermissionsIndex(tp3, id)
	tp3Ref := resource.ReferenceFromReferenceOrID(tp3.GetId())

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was updated
	ctpResource = suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctpResource, []*pbauth.Permission{p2}, []*pbauth.Permission{p1, p3}, []*pbresource.Reference{
		tp1Ref,
		tp2Ref,
		tp3Ref,
	})
	rtest.RequireOwner(suite.T(), ctpResource, wi.Id, true)
	assertCTPDefaultStatus(suite.T(), ctpResource, false)

	// Delete the traffic permissions without updating the caches. Ensure is default is right even when the caches contain stale data.
	suite.client.MustDelete(suite.T(), tp1.Id)
	suite.client.MustDelete(suite.T(), tp2.Id)
	suite.client.MustDelete(suite.T(), tp3.Id)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	ctpResource = suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctpResource, []*pbauth.Permission{}, []*pbauth.Permission{}, []*pbresource.Reference{})
	rtest.RequireOwner(suite.T(), ctpResource, wi.Id, true)
	assertCTPDefaultStatus(suite.T(), ctpResource, true)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsDelete_DestinationWorkloadIdentityExists() {
	// create the workload identity to be referenced
	wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, wi.Id.Name).WithTenancy(resource.DefaultNamespacedTenancy()).WithOwner(wi.Id).ID()

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
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
	}).Write(suite.T(), suite.client)
	suite.requireTrafficPermissionsIndex(tp1, id)
	tp1Ref := resource.ReferenceFromReferenceOrID(tp1.GetId())
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
	suite.requireTrafficPermissionsIndex(tp2, id)
	tp2Ref := resource.ReferenceFromReferenceOrID(tp2.GetId())

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	ctp := suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{p2}, []*pbauth.Permission{p1}, []*pbresource.Reference{tp1Ref, tp2Ref})
	rtest.RequireOwner(suite.T(), ctp, wi.Id, true)

	// Delete TP2
	suite.client.MustDelete(suite.T(), tp2.Id)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	// Ensure that the CTP was updated
	ctp = suite.client.RequireResourceExists(suite.T(), id)
	suite.requireCTP(ctp, []*pbauth.Permission{}, []*pbauth.Permission{p1}, []*pbresource.Reference{
		tp1Ref,
	})

	// Ensure TP2 is untracked
	// newTps := tpm.MapTrafficPermissions.GetTrafficPermissionsForCTP(ctp.Id)
	// require.Len(suite.T(), newTps, 1)
	// require.Equal(suite.T(), newTps[0].Name, tp1.Id.Name)
}

func (suite *controllerSuite) TestReconcile_TrafficPermissionsDelete_DestinationWorkloadIdentityDoesNotExist() {
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
	}).Write(suite.T(), suite.client)
	wi1ID := &pbresource.ID{
		Name:    "wi1",
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp1.Id.Tenancy,
	}
	suite.requireTrafficPermissionsIndex(tp1, wi1ID)
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
	suite.requireTrafficPermissionsIndex(tp2, wi1ID)

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
	mgr.Register(Controller(tpm.MapTrafficPermissions))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	// Add a workload identity
	workloadIdentity := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)

	// Wait for the controller to record that the CTP has been computed
	res := suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity.Id), StatusKey)
	// Check that the status was updated
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1", true))

	// Check that the CTP resource exists and contains no permissions
	ctpID := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi1").ID()
	ctpObject := suite.client.RequireResourceExists(suite.T(), ctpID)
	suite.requireCTP(ctpObject, nil, nil, nil)

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
	tp1Ref := resource.ReferenceFromReferenceOrID(tp1.GetId())
	// Wait for the controller to record that the CTP has been re-computed
	suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity.Id), StatusKey)
	// Check that the ctp has been regenerated
	ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
	rtest.RequireStatusCondition(suite.T(), ctpObject, StatusKey, ConditionComputed("wi1", false))
	// check wi1
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1}, nil, []*pbresource.Reference{tp1Ref})

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
	tp2Ref := resource.ReferenceFromReferenceOrID(tp2.GetId())
	// check wi1 is the same
	ctpObject = suite.client.RequireResourceExists(suite.T(), ctpID)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1}, nil, []*pbresource.Reference{tp1Ref})
	// check no ctp2
	ctpID2 := rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi2").ID()
	suite.client.RequireResourceNotFound(suite.T(), ctpID2)

	// delete tp1
	suite.client.MustDelete(suite.T(), tp1.Id)
	suite.client.WaitForDeletion(suite.T(), tp1.Id)
	// check wi1 has no permissions
	ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
	suite.requireCTP(ctpObject, nil, nil, nil)

	// edit tp2 to point to wi1
	rtest.Resource(pbauth.TrafficPermissionsType, "tp2").WithData(suite.T(), &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{p2},
	}).Write(suite.T(), suite.client)
	// check wi1 has tp2's permissions
	ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpID, ctpObject.Version)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p2}, nil, []*pbresource.Reference{tp2Ref})
	// check no ctp2
	ctpID2 = rtest.Resource(pbauth.ComputedTrafficPermissionsType, "wi2").ID()
	suite.client.RequireResourceNotFound(suite.T(), ctpID2)
}

func (suite *controllerSuite) TestControllerMultipleTrafficPermissions() {
	// TODO: refactor this, turn back on once timing flakes are understood
	suite.T().Skip("flaky behavior observed")
	// In this test we check operations for a workload identity and multiple referencing traffic permissions
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller(tpm.MapTrafficPermissions))
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
	suite.requireTrafficPermissionsIndex(tp1, wi1ID)
	tp1Ref := resource.ReferenceFromReferenceOrID(tp1.GetId())
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
	suite.requireTrafficPermissionsIndex(tp1, wi1ID)
	tp2Ref := resource.ReferenceFromReferenceOrID(tp2.GetId())

	// Add a workload identity
	workloadIdentity := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	ctpID := resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity.Id)
	// Wait for the controller to record that the CTP has been computed
	res := suite.client.WaitForReconciliation(suite.T(), ctpID, StatusKey)
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1", false))
	// check ctp1 has tp1 and tp2
	ctpObject := suite.client.RequireResourceExists(suite.T(), res.Id)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, nil, []*pbresource.Reference{
		tp1Ref,
		tp2Ref,
	})

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
	tp3Ref := resource.ReferenceFromReferenceOrID(tp3.GetId())

	// check ctp1 has tp3
	ctpObject = suite.client.WaitForReconciliation(suite.T(), ctpObject.Id, StatusKey)
	ctpObject = suite.client.WaitForNewVersion(suite.T(), ctpObject.Id, ctpObject.Version)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3}, []*pbresource.Reference{
		tp1Ref, tp2Ref, tp3Ref,
	})

	// delete ctp
	suite.client.MustDelete(suite.T(), ctpObject.Id)
	suite.client.WaitForDeletion(suite.T(), ctpObject.Id)
	// check ctp regenerated, has all permissions
	res = suite.client.WaitForReconciliation(suite.T(), ctpID, StatusKey)
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1", false))
	ctpObject = suite.client.RequireResourceExists(suite.T(), res.Id)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3}, []*pbresource.Reference{
		tp1Ref, tp2Ref, tp3Ref,
	})

	// delete wi1
	suite.client.MustDelete(suite.T(), workloadIdentity.Id)
	suite.client.WaitForDeletion(suite.T(), workloadIdentity.Id)

	// recreate wi1
	rtest.Resource(pbauth.WorkloadIdentityType, "wi1").Write(suite.T(), suite.client)
	// check ctp regenerated, has all permissions
	res = suite.client.WaitForReconciliation(suite.T(), ctpID, StatusKey)
	rtest.RequireStatusCondition(suite.T(), res, StatusKey, ConditionComputed("wi1", false))
	ctpObject = suite.client.RequireResourceExists(suite.T(), res.Id)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3}, []*pbresource.Reference{
		tp1Ref, tp2Ref, tp3Ref,
	})

	// delete tp3
	suite.client.MustDelete(suite.T(), tp3.Id)
	suite.client.WaitForDeletion(suite.T(), tp3.Id)
	suite.client.RequireResourceNotFound(suite.T(), tp3.Id)
	// check ctp1 has tp1 and tp2, and not tp3
	res = suite.client.WaitForReconciliation(suite.T(), ctpObject.Id, StatusKey)
	ctpObject = suite.client.WaitForNewVersion(suite.T(), res.Id, ctpObject.Version)
	suite.requireCTP(ctpObject, []*pbauth.Permission{p1, p2}, nil, []*pbresource.Reference{
		tp1Ref, tp2Ref,
	})

	// add wi2
	workloadIdentity2 := rtest.Resource(pbauth.WorkloadIdentityType, "wi2").Write(suite.T(), suite.client)
	// Wait for the controller to record that the CTP has been computed
	res2 := suite.client.WaitForReconciliation(suite.T(), resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, workloadIdentity2.Id), StatusKey)
	rtest.RequireStatusCondition(suite.T(), res2, StatusKey, ConditionComputed("wi2", false))
	// check ctp2 has no permissions
	ctpObject2 := suite.client.RequireResourceExists(suite.T(), res2.Id)
	suite.requireCTP(ctpObject2, nil, nil, nil)

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
		suite.requireCTP(res, []*pbauth.Permission{p1, p2}, []*pbauth.Permission{p3}, []*pbresource.Reference{
			tp1Ref, tp2Ref, tp3Ref,
		})
	})
	// check wi1 has no permissions after 6 reconciles
	ctpObject = suite.client.WaitForReconciliation(suite.T(), ctpObject.Id, StatusKey)
	res = suite.client.WaitForReconciliation(suite.T(), ctpObject.Id, StatusKey)
	suite.client.WaitForResourceState(suite.T(), res.Id, func(t rtest.T, res *pbresource.Resource) {
		suite.requireCTP(res, nil, nil, nil)
	})
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func assertCTPDefaultStatus(t *testing.T, resource *pbresource.Resource, isDefault bool) {
	dec := rtest.MustDecode[*pbauth.ComputedTrafficPermissions](t, resource)
	require.Equal(t, isDefault, dec.Data.IsDefault)
}
