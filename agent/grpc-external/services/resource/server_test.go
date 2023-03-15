package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
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
	// leave registry empty
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
	require.Contains(t, err.Error(), "resource type mesh/v1/service not registered")
}

func TestRead_ResourceNotFound(t *testing.T) {
	server := NewServer(testServerConfig(t))
	client := testClient(t, server)

	// prime registry with a type
	serviceType := &pbresource.Type{Group: "mesh", GroupVersion: "v1", Kind: "service"}
	server.registry.Register(resource.Registration{Type: serviceType})

	_, err := client.Read(context.Background(), &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Uid:     "abcd",
			Name:    "billing",
			Type:    serviceType,
			Tenancy: &pbresource.Tenancy{Partition: "default", Namespace: "default", PeerName: "default"},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource not found")
}

func TestRead_GroupVersionMismatch(t *testing.T) {
	ctx := context.Background()
	server := NewServer(testServerConfig(t))
	client := testClient(t, server)

	// prime registry with a type
	v1Type := &pbresource.Type{Group: "mesh", GroupVersion: "v1", Kind: "service"}
	v2Type := &pbresource.Type{Group: "mesh", GroupVersion: "v2", Kind: "service"}
	server.registry.Register(resource.Registration{Type: v1Type})
	server.registry.Register(resource.Registration{Type: v2Type})
	someTenancy := &pbresource.Tenancy{Partition: "default", Namespace: "default", PeerName: "default"}

	// prime backend with a resource of GroupVersion v1
	_, err := server.backend.WriteCAS(
		ctx,
		&pbresource.Resource{
			Id: &pbresource.ID{
				Uid:     "someUid",
				Name:    "someName",
				Type:    v1Type,
				Tenancy: someTenancy,
			},
		},
		"",
	)
	require.NoError(t, err)

	// read same resource with GroupVersion v2 should fail
	_, err = client.Read(ctx, &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Uid:     "someUid",
			Name:    "someName",
			Type:    v2Type,
			Tenancy: someTenancy,
		},
	})
	require.Error(t, err)
	// TODO: how do i stronger match on the error type instead of a string?
	require.Contains(t, err.Error(), "InvalidArgument")
	require.Contains(t, err.Error(), "resource was requested with GroupVersion")
}

func TestRead_Success(t *testing.T) {
	server := NewServer(testServerConfig(t))
	client := testClient(t, server)

	// init registry with a type
	v1Type := &pbresource.Type{Group: "mesh", GroupVersion: "v1", Kind: "service"}
	server.registry.Register(resource.Registration{Type: v1Type})

	// init backend with a resource
	someTenancy := &pbresource.Tenancy{Partition: "default", Namespace: "default", PeerName: "default"}
	someId := &pbresource.ID{
		Uid:     "someUid",
		Name:    "someName",
		Type:    v1Type,
		Tenancy: someTenancy,
	}
	expected, err := server.backend.WriteCAS(context.Background(), &pbresource.Resource{Id: someId}, "")
	require.NoError(t, err)

	// verify same resource read
	resp, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: someId})
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, resp.Resource, expected)
}

func TestIsConsistentRead_True(t *testing.T) {
	require.True(t, isConsistentRead(metadata.NewIncomingContext(
		context.Background(),
		metadata.New(map[string]string{"x-consul-consistency-mode": "consistent"}),
	)))
}

func TestIsConsistentRead_False(t *testing.T) {
	// no metadata
	require.False(t, isConsistentRead(context.Background()))

	// empty string
	require.False(t, isConsistentRead(metadata.NewIncomingContext(
		context.Background(),
		metadata.New(map[string]string{"x-consul-consistency-mode": ""}),
	)))

	// no match string
	require.False(t, isConsistentRead(metadata.NewIncomingContext(
		context.Background(),
		metadata.New(map[string]string{"x-consul-consistency-mode": "blah"}),
	)))
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
