// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package demo

import (
	"testing"

	"github.com/stretchr/testify/require"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestArtistReconciler(t *testing.T) {
	client := svctest.RunResourceService(t, RegisterTypes)

	// Seed the database with an artist.
	res, err := GenerateV2Artist()
	require.NoError(t, err)

	// Set the genre to BLUES to ensure there are 10 albums.
	var artist pbdemov2.Artist
	require.NoError(t, res.Data.UnmarshalTo(&artist))
	artist.Genre = pbdemov2.Genre_GENRE_BLUES
	require.NoError(t, res.Data.MarshalFrom(&artist))

	ctx := testutil.TestContext(t)
	writeRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Call the reconciler for that artist.
	var rec artistReconciler
	runtime := controller.Runtime{
		Client: client,
		Logger: testutil.Logger(t),
	}
	req := controller.Request{
		ID: writeRsp.Resource.Id,
	}
	require.NoError(t, rec.Reconcile(ctx, runtime, req))

	// Check the status was updated.
	readRsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: writeRsp.Resource.Id})
	require.NoError(t, err)
	require.Contains(t, readRsp.Resource.Status, "consul.io/artist-controller")

	status := readRsp.Resource.Status["consul.io/artist-controller"]
	require.Equal(t, writeRsp.Resource.Generation, status.ObservedGeneration)
	require.Len(t, status.Conditions, 11)
	require.Equal(t, "Accepted", status.Conditions[0].Type)
	require.Equal(t, "AlbumCreated", status.Conditions[1].Type)

	// Check the albums were created.
	listRsp, err := client.List(ctx, &pbresource.ListRequest{
		Type:    TypeV2Album,
		Tenancy: readRsp.Resource.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 10)

	// Delete an album.
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: listRsp.Resources[0].Id})
	require.NoError(t, err)

	// Call the reconciler again.
	require.NoError(t, rec.Reconcile(ctx, runtime, req))

	// Check the album was recreated.
	listRsp, err = client.List(ctx, &pbresource.ListRequest{
		Type:    TypeV2Album,
		Tenancy: readRsp.Resource.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 10)

	// Set the genre to DISCO.
	readRsp, err = client.Read(ctx, &pbresource.ReadRequest{Id: writeRsp.Resource.Id})
	require.NoError(t, err)

	res = readRsp.Resource
	require.NoError(t, res.Data.UnmarshalTo(&artist))
	artist.Genre = pbdemov2.Genre_GENRE_DISCO
	require.NoError(t, res.Data.MarshalFrom(&artist))

	_, err = client.Write(ctx, &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Call the reconciler again.
	require.NoError(t, rec.Reconcile(ctx, runtime, req))

	// Check there are only 3 albums now.
	listRsp, err = client.List(ctx, &pbresource.ListRequest{
		Type:    TypeV2Album,
		Tenancy: readRsp.Resource.Id.Tenancy,
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 3)
}
