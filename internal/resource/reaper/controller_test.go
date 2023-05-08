// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package reaper

import (
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	// Retrieve the tombstone
	listRsp, err := client.List(ctx, &pbresource.ListRequest{
		Type:    resource.TypeV1Tombstone,
		Tenancy: writeRsp.Resource.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 1)
	tombstone := listRsp.Resources[0]

	// Reconcile the tombstone
	var rec tombstoneReconciler
	runtime := controller.Runtime{
		Client: client,
		Logger: testutil.Logger(t),
	}
	req := controller.Request{ID: tombstone.Id}
	require.NoError(t, rec.Reconcile(ctx, runtime, req))

	// Verify tombstone deleted
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

	// Reconcile the tombstone
	var rec tombstoneReconciler
	runtime := controller.Runtime{
		Client: client,
		Logger: testutil.Logger(t),
	}
	req := controller.Request{ID: tombstone.Id}
	require.NoError(t, rec.Reconcile(ctx, runtime, req))

	// Verify 3 albums deleted
	listRsp, err = client.List(ctx, &pbresource.ListRequest{
		Type:    demo.TypeV2Album,
		Tenancy: artist.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Empty(t, listRsp.Resources)

	// Verify artist tombstone deleted
	_, err = client.Read(ctx, &pbresource.ReadRequest{Id: tombstone.Id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound.String(), status.Code(err).String())

	// Verify 3 albums' tombstones created
	listRsp, err = client.List(ctx, &pbresource.ListRequest{
		Type:    resource.TypeV1Tombstone,
		Tenancy: artist.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, numAlbums)
}
