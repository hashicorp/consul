// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestWrite_TODO(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	resp, err := client.Write(context.Background(), &pbresource.WriteRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

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

	registry := resource.NewRegistry()
	return NewServer(Config{registry: registry, Backend: backend})
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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

var (
	tenancy = &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
		PeerName:  "local",
	}
	typev1 = &pbresource.Type{
		Group:        "mesh",
		GroupVersion: "v1",
		Kind:         "service",
	}
	typev2 = &pbresource.Type{
		Group:        "mesh",
		GroupVersion: "v2",
		Kind:         "service",
	}
	id1 = &pbresource.ID{
		Uid:     "abcd",
		Name:    "billing",
		Type:    typev1,
		Tenancy: tenancy,
	}
	id2 = &pbresource.ID{
		Uid:     "abcd",
		Name:    "billing",
		Type:    typev2,
		Tenancy: tenancy,
	}
	resourcev1 = &pbresource.Resource{
		Id: &pbresource.ID{
			Uid:     "someUid",
			Name:    "someName",
			Type:    typev1,
			Tenancy: tenancy,
		},
		Version: "",
	}
)
