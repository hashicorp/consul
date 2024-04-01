// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancytest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/tenancy/internal/controllers/common"
	"github.com/hashicorp/consul/internal/tenancy/internal/controllers/namespace"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// Due to a circular dependency, this test can't reside in the package next to the controller it is testing.

type nsTestSuite struct {
	suite.Suite

	client  *rtest.Client
	runtime controller.Runtime
	ctx     context.Context
}

func (ts *nsTestSuite) SetupTest() {
	builder := svctest.NewResourceServiceBuilder().
		WithV2Tenancy(true).
		WithRegisterFns(demo.RegisterTypes)
	ts.client = rtest.NewClient(builder.Run(ts.T()))
	ts.runtime = controller.Runtime{Client: ts.client, Logger: testutil.Logger(ts.T())}
	ts.ctx = testutil.TestContext(ts.T())

	mgr := controller.NewManager(ts.client, testutil.Logger(ts.T()))
	mgr.Register(namespace.Controller(builder.Registry()))
	mgr.SetRaftLeader(true)
	ctx, cancel := context.WithCancel(context.Background())
	ts.T().Cleanup(cancel)
	go mgr.Run(ctx)
}

func (ts *nsTestSuite) waitForReconciliation(id *pbresource.ID, reason string) {
	ts.T().Helper()

	retry.Run(ts.T(), func(r *retry.R) {
		rsp, err := ts.client.Read(context.Background(), &pbresource.ReadRequest{
			Id: id,
		})
		require.NoError(r, err)

		status, found := rsp.Resource.Status[namespace.StatusKey]
		require.True(r, found)
		require.Len(r, status.Conditions, 1)
		require.Equal(r, reason, status.Conditions[0].Reason)
	})
}

func (ts *nsTestSuite) TestNamespaceController_HappyPath() {
	// Create namespace ns1
	ns1 := rtest.Resource(pbtenancy.NamespaceType, "ns1").
		// Keep this CE friendly by using default partition
		WithTenancy(resource.DefaultPartitionedTenancy()).
		WithData(ts.T(), &pbtenancy.Namespace{Description: "namespace ns1"}).
		Write(ts.T(), ts.client)

	// Wait for it to be accepted
	ts.waitForReconciliation(ns1.Id, common.ReasonAcceptedOK)

	// Verify namespace finalizer added
	ns1 = ts.client.RequireResourceMeta(ts.T(), ns1.Id, resource.FinalizerKey, namespace.StatusKey)

	// Add a namespace scoped tenant to the namespace
	artist1 := rtest.Resource(demo.TypeV1Artist, "moonchild").
		WithTenancy(&pbresource.Tenancy{
			Partition: resource.DefaultPartitionName,
			Namespace: ns1.Id.Name,
		}).
		WithData(ts.T(), &pbdemo.Artist{Name: "Moonchild"}).
		Write(ts.T(), ts.client)

	// Delete the namespace
	_, err := ts.client.Delete(ts.ctx, &pbresource.DeleteRequest{Id: ns1.Id})
	require.NoError(ts.T(), err)

	// Wait for the namespace to be deleted
	ts.client.WaitForDeletion(ts.T(), ns1.Id)

	// Verify tenants deleted.
	ts.client.RequireResourceNotFound(ts.T(), artist1.Id)
}

func (ts *nsTestSuite) TestNamespaceController_DeleteBlockedByTenantsWithFinalizers() {
	// Create namespace ns1
	ns1 := rtest.Resource(pbtenancy.NamespaceType, "ns1").
		WithTenancy(resource.DefaultPartitionedTenancy()).
		WithData(ts.T(), &pbtenancy.Namespace{Description: "namespace ns1"}).
		Write(ts.T(), ts.client)

	// Wait for it to be accepted
	ts.waitForReconciliation(ns1.Id, common.ReasonAcceptedOK)

	// Add artist to namespace
	_ = rtest.Resource(demo.TypeV1Artist, "weezer").
		WithTenancy(&pbresource.Tenancy{
			Partition: resource.DefaultPartitionName,
			Namespace: ns1.Id.Name,
		}).
		WithData(ts.T(), &pbdemo.Artist{Name: "Weezer"}).
		Write(ts.T(), ts.client)

	// Add another artist to namespace with a finalizer so that is blocks namespace deletion.
	artist2 := rtest.Resource(demo.TypeV1Artist, "foofighters").
		WithTenancy(&pbresource.Tenancy{
			Partition: resource.DefaultPartitionName,
			Namespace: ns1.Id.Name,
		}).
		WithData(ts.T(), &pbdemo.Artist{Name: "Foo Fighters"}).
		WithMeta(resource.FinalizerKey, "finalizer2").
		Write(ts.T(), ts.client)

	// Delete the namespace - this activates the controller logic to delete all tenants
	ts.client.Delete(ts.ctx, &pbresource.DeleteRequest{Id: ns1.Id})

	// Delete should be blocked by artist2 tenant with finalizer
	ts.client.WaitForStatusConditionAnyGen(ts.T(), ns1.Id, namespace.StatusKey, &pbresource.Condition{
		Type:    common.ConditionDeleted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  common.ReasonDeletionInProgress,
		Message: common.ErrStillHasTenants.Error(),
	})

	// Remove the finalizer on artist2 to unblock deletion of ns1
	artist2 = ts.client.RequireResourceExists(ts.T(), artist2.Id)
	resource.RemoveFinalizer(artist2, "finalizer2")
	_, err := ts.client.Write(ts.ctx, &pbresource.WriteRequest{Resource: artist2})
	require.NoError(ts.T(), err)

	// The final reconcile should delete artist since it was marked for deletion and
	// and has no finalizers. Given no more tenants, wait for namespace to be deleted.
	ts.client.WaitForDeletion(ts.T(), ns1.Id)
}

func TestNamespaceControllerSuite(t *testing.T) {
	suite.Run(t, new(nsTestSuite))
}
