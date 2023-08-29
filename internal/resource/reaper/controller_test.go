// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reaper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestReconcile_ResourceWithNoChildren(t *testing.T) {
	client := svctest.RunResourceService(t, demo.RegisterTypes)

	// Seed the database with an artist.
	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	ctx := testutil.TestContext(t)
	writeRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Delete the artist to create a tombstone
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: writeRsp.Resource.Id})
	require.NoError(t, err)

	// Retrieve tombstone
	listRsp, err := client.List(ctx, &pbresource.ListRequest{
		Type:    resource.TypeV1Tombstone,
		Tenancy: writeRsp.Resource.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 1)
	tombstone := listRsp.Resources[0]

	// Verify reconcile does first pass and queues up for a second pass
	rec := newReconciler()
	runtime := controller.Runtime{
		Client: client,
		Logger: testutil.Logger(t),
	}
	req := controller.Request{ID: tombstone.Id}
	require.ErrorIs(t, controller.RequeueAfterError(secondPassDelay), rec.Reconcile(ctx, runtime, req))

	// Verify condition FirstPassCompleted is true
	readRsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: tombstone.Id})
	require.NoError(t, err)
	tombstone = readRsp.Resource
	condition := tombstone.Status[statusKeyReaperController].Conditions[0]
	require.Equal(t, conditionTypeFirstPassCompleted, condition.Type)
	require.Equal(t, pbresource.Condition_STATE_TRUE, condition.State)

	// Verify reconcile does second pass and tombstone is deleted
	// Fake out time so elapsed time > secondPassDelay
	rec.timeNow = func() time.Time { return time.Now().Add(secondPassDelay + time.Second) }
	require.NoError(t, rec.Reconcile(ctx, runtime, req))
	_, err = client.Read(ctx, &pbresource.ReadRequest{Id: tombstone.Id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound.String(), status.Code(err).String())

	// Reconcile again to verify no-op on an already deleted tombstone
	require.NoError(t, rec.Reconcile(ctx, runtime, req))
}

func TestReconcile_ResourceWithChildren(t *testing.T) {
	client := svctest.RunResourceService(t, demo.RegisterTypes)

	// Seed the database with an artist
	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	ctx := testutil.TestContext(t)
	writeRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	artist := writeRsp.Resource

	// Create 3 albums owned by the artist
	numAlbums := 3
	for i := 0; i < numAlbums; i++ {
		res, err = demo.GenerateV2Album(artist.Id)
		require.NoError(t, err)
		_, err := client.Write(ctx, &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)
	}

	// Delete the artist to create a tombstone
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: writeRsp.Resource.Id})
	require.NoError(t, err)

	// Retrieve the tombstone
	listRsp, err := client.List(ctx, &pbresource.ListRequest{
		Type:    resource.TypeV1Tombstone,
		Tenancy: writeRsp.Resource.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 1)
	tombstone := listRsp.Resources[0]

	// Verify reconcile does first pass delete and queues up for a second pass
	rec := newReconciler()
	runtime := controller.Runtime{
		Client: client,
		Logger: testutil.Logger(t),
	}
	req := controller.Request{ID: tombstone.Id}
	require.ErrorIs(t, controller.RequeueAfterError(secondPassDelay), rec.Reconcile(ctx, runtime, req))

	// Verify 3 albums deleted
	listRsp, err = client.List(ctx, &pbresource.ListRequest{
		Type:    demo.TypeV2Album,
		Tenancy: artist.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Empty(t, listRsp.Resources)

	// Verify condition FirstPassCompleted is true
	readRsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: tombstone.Id})
	require.NoError(t, err)
	tombstone = readRsp.Resource
	condition := tombstone.Status[statusKeyReaperController].Conditions[0]
	require.Equal(t, conditionTypeFirstPassCompleted, condition.Type)
	require.Equal(t, pbresource.Condition_STATE_TRUE, condition.State)

	// Verify reconcile does second pass
	// Fake out time so elapsed time > secondPassDelay
	rec.timeNow = func() time.Time { return time.Now().Add(secondPassDelay + time.Second) }
	require.NoError(t, rec.Reconcile(ctx, runtime, req))

	// Verify artist tombstone deleted
	_, err = client.Read(ctx, &pbresource.ReadRequest{Id: tombstone.Id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound.String(), status.Code(err).String())

	// Verify tombstones for 3 albums created
	listRsp, err = client.List(ctx, &pbresource.ListRequest{
		Type:    resource.TypeV1Tombstone,
		Tenancy: artist.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, numAlbums)
}

func TestReconcile_RequeueWithDelayWhenSecondPassDelayNotElapsed(t *testing.T) {
	client := svctest.RunResourceService(t, demo.RegisterTypes)

	// Seed the database with an artist.
	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	ctx := testutil.TestContext(t)
	writeRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Delete the artist to create a tombstone
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: writeRsp.Resource.Id})
	require.NoError(t, err)

	// Retrieve tombstone
	listRsp, err := client.List(ctx, &pbresource.ListRequest{
		Type:    resource.TypeV1Tombstone,
		Tenancy: writeRsp.Resource.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 1)
	tombstone := listRsp.Resources[0]

	// Verify reconcile does first pass and queues up for a second pass
	rec := newReconciler()
	runtime := controller.Runtime{
		Client: client,
		Logger: testutil.Logger(t),
	}
	req := controller.Request{ID: tombstone.Id}
	require.ErrorIs(t, controller.RequeueAfterError(secondPassDelay), rec.Reconcile(ctx, runtime, req))

	// Verify condition FirstPassCompleted is true
	readRsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: tombstone.Id})
	require.NoError(t, err)
	tombstone = readRsp.Resource
	condition := tombstone.Status[statusKeyReaperController].Conditions[0]
	require.Equal(t, conditionTypeFirstPassCompleted, condition.Type)
	require.Equal(t, pbresource.Condition_STATE_TRUE, condition.State)

	// Verify requeued for second pass since secondPassDelay time has not elapsed
	require.ErrorIs(t, controller.RequeueAfterError(secondPassDelay), rec.Reconcile(ctx, runtime, req))
}
