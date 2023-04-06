// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestWriteStatus_TODO(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	resp, err := client.WriteStatus(context.Background(), &pbresource.WriteStatusRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestDelete_TODO(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	resp, err := client.Delete(context.Background(), &pbresource.DeleteRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func testServer(t *testing.T) *Server {
	t.Helper()

	backend, err := inmem.NewBackend()
	require.NoError(t, err)
	go backend.Run(testContext(t))

	return NewServer(Config{
		Logger:   testutil.Logger(t),
		Registry: resource.NewRegistry(),
		Backend:  backend,
	})
}

func testClient(t *testing.T, server *Server) pbresource.ResourceServiceClient {
	t.Helper()

	addr := testutils.RunTestServer(t, server)

	//nolint:staticcheck
	conn, err := grpc.DialContext(context.Background(), addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return pbresource.NewResourceServiceClient(conn)
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func modifyArtist(t *testing.T, res *pbresource.Resource) *pbresource.Resource {
	t.Helper()

	var artist pbdemov2.Artist
	require.NoError(t, res.Data.UnmarshalTo(&artist))
	artist.Name = fmt.Sprintf("The artist formerly known as %s", artist.Name)

	data, err := anypb.New(&artist)
	require.NoError(t, err)

	res = clone(res)
	res.Data = data
	return res
}
