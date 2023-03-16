package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
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

func TestRead_TypeNotFound(t *testing.T) {
	mockRegistry := &MockRegistry{}
	mockRegistry.On("Resolve", mock.Anything).Return(resource.Registration{}, false)
	server := NewServer(Config{registry: mockRegistry})
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
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type mesh/v1/service not registered")
}

type readTestCase struct {
	readFn string
	ctx    context.Context
}

func readTestCases() map[string]readTestCase {
	return map[string]readTestCase{
		"read": {
			readFn: "Read",
			ctx:    context.Background(),
		},
		"consistent read": {
			readFn: "ReadConsistent",
			ctx: metadata.NewOutgoingContext(
				context.Background(),
				metadata.New(map[string]string{"x-consul-consistency-mode": "consistent"}),
			),
		},
	}

}

func TestRead_ResourceNotFound(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			mockRegistry := &MockRegistry{}
			mockRegistry.On("Resolve", mock.Anything).Return(resource.Registration{}, true)

			mockBackend := &MockBackend{}
			mockBackend.On(tc.readFn, mock.Anything, mock.Anything).Return(nil, storage.ErrNotFound)

			server := NewServer(Config{
				registry: mockRegistry,
				backend:  mockBackend,
			})
			client := testClient(t, server)

			_, err := client.Read(tc.ctx, &pbresource.ReadRequest{
				Id: &pbresource.ID{
					Uid:     "abcd",
					Name:    "billing",
					Type:    &pbresource.Type{Group: "mesh", GroupVersion: "v1", Kind: "service"},
					Tenancy: &pbresource.Tenancy{Partition: "default", Namespace: "default", PeerName: "default"},
				},
			})
			require.Error(t, err)
			require.Equal(t, codes.NotFound.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource not found")
			mockBackend.AssertCalled(t, tc.readFn, mock.Anything, mock.Anything)
		})
	}
}

func TestRead_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			mockRegistry := &MockRegistry{}
			mockRegistry.On("Resolve", mock.Anything).Return(resource.Registration{}, true)

			mockBackend := &MockBackend{}
			mockBackend.On(tc.readFn, mock.Anything, mock.Anything).Return(nil, storage.GroupVersionMismatchError{
				RequestedType: &pbresource.Type{GroupVersion: "v2"},
				Stored:        &pbresource.Resource{Id: &pbresource.ID{Type: &pbresource.Type{GroupVersion: "v1"}}},
			})

			server := NewServer(Config{
				registry: mockRegistry,
				backend:  mockBackend,
			})
			client := testClient(t, server)

			_, err := client.Read(tc.ctx, &pbresource.ReadRequest{
				Id: &pbresource.ID{
					Uid:     "abcd",
					Name:    "billing",
					Type:    &pbresource.Type{Group: "mesh", GroupVersion: "v2", Kind: "service"},
					Tenancy: &pbresource.Tenancy{Partition: "default", Namespace: "default", PeerName: "default"},
				},
			})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource was requested with GroupVersion")
			mockBackend.AssertCalled(t, tc.readFn, mock.Anything, mock.Anything)
		})
	}
}

func TestRead_Success(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			mockRegistry := &MockRegistry{}
			mockRegistry.On("Resolve", mock.Anything).Return(resource.Registration{}, true)

			typ := &pbresource.Type{
				Group:        "mesh",
				GroupVersion: "v1",
				Kind:         "service",
			}
			id := &pbresource.ID{
				Uid:     "someUid",
				Name:    "someName",
				Type:    typ,
				Tenancy: &pbresource.Tenancy{},
			}
			resource := &pbresource.Resource{
				Id: id,
			}

			mockBackend := &MockBackend{}
			mockBackend.On(tc.readFn, mock.Anything, mock.Anything).Return(resource, nil)

			server := NewServer(Config{
				registry: mockRegistry,
				backend:  mockBackend,
			})
			client := testClient(t, server)

			_, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: id})
			require.NoError(t, err)
			mockBackend.AssertCalled(t, tc.readFn, mock.Anything, mock.Anything)
		})
	}
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
