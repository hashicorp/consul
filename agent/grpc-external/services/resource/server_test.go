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

func testServerConfig(t *testing.T) Config {
	backend, err := inmem.NewBackend()
	require.NoError(t, err)
	return Config{
		registry: resource.NewRegistry(),
		backend:  backend,
	}
}

func TestRead_TypeNotFound(t *testing.T) {
	server := NewServer(testServerConfig(t))
	client := testClient(t, server)
	_, err := client.Read(context.Background(), &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Uid:  "abcd",
			Name: "billing",
			Type: &pbresource.Type{
				Group:        "mesh",
				GroupVersion: "v1",
				Kind:         "service",
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource type mesh/v1/service not found")

}

func TestWrite_TODO(t *testing.T) {
	server := NewServer(Config{})
	client := testClient(t, server)
	resp, err := client.Write(context.Background(), &pbresource.WriteRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestWriteStatus_TODO(t *testing.T) {
	server := NewServer(Config{})
	client := testClient(t, server)
	resp, err := client.WriteStatus(context.Background(), &pbresource.WriteStatusRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestList_TODO(t *testing.T) {
	server := NewServer(Config{})
	client := testClient(t, server)
	resp, err := client.List(context.Background(), &pbresource.ListRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestDelete_TODO(t *testing.T) {
	server := NewServer(Config{})
	client := testClient(t, server)
	resp, err := client.Delete(context.Background(), &pbresource.DeleteRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestWatch_TODO(t *testing.T) {
	server := NewServer(Config{})
	client := testClient(t, server)
	wc, err := client.Watch(context.Background(), &pbresource.WatchRequest{})
	require.NoError(t, err)
	require.NotNil(t, wc)
}
