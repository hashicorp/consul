package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestRead_TypeNotFound(t *testing.T) {
	server := NewServer(Config{registry: resource.NewRegistry()})
	client := testClient(t, server)

	_, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: id1})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type mesh/v1/service not registered")
}

func TestRead_ResourceNotFound(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			server.registry.Register(resource.Registration{Type: typev1})
			client := testClient(t, server)

			_, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: id1})
			require.Error(t, err)
			require.Equal(t, codes.NotFound.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource not found")
		})
	}
}

func TestRead_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			server.registry.Register(resource.Registration{Type: typev1})
			server.registry.Register(resource.Registration{Type: typev2})
			client := testClient(t, server)

			resource1 := &pbresource.Resource{Id: id1, Version: ""}
			_, err := server.backend.WriteCAS(tc.ctx, resource1)
			require.NoError(t, err)

			_, err = client.Read(tc.ctx, &pbresource.ReadRequest{Id: id2})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource was requested with GroupVersion")
		})
	}
}

func TestRead_Success(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			server.registry.Register(resource.Registration{Type: typev1})
			client := testClient(t, server)
			resource1 := &pbresource.Resource{Id: id1, Version: ""}
			resource1, err := server.backend.WriteCAS(tc.ctx, resource1)
			require.NoError(t, err)

			rsp, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: id1})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, resource1, rsp.Resource)
		})
	}
}

func TestRead_VerifyReadConsistencyArg(t *testing.T) {
	// Uses a mockBackend instead of the inmem Backend to verify the ReadConsistency argument is set correctly.
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			mockBackend := NewMockBackend(t)
			server := NewServer(Config{
				registry: resource.NewRegistry(),
				backend:  mockBackend,
			})
			server.registry.Register(resource.Registration{Type: typev1})
			resource1 := &pbresource.Resource{Id: id1, Version: "1"}
			mockBackend.On("Read", mock.Anything, mock.Anything, mock.Anything).Return(resource1, nil)
			client := testClient(t, server)

			rsp, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: id1})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, resource1, rsp.Resource)
			mockBackend.AssertCalled(t, "Read", mock.Anything, tc.consistency, mock.Anything)
		})
	}
}

type readTestCase struct {
	consistency storage.ReadConsistency
	ctx         context.Context
}

func readTestCases() map[string]readTestCase {
	return map[string]readTestCase{
		"eventually consistent read": {
			consistency: storage.EventualConsistency,
			ctx:         context.Background(),
		},
		"strongly consistent read": {
			consistency: storage.StrongConsistency,
			ctx: metadata.NewOutgoingContext(
				context.Background(),
				metadata.New(map[string]string{"x-consul-consistency-mode": "consistent"}),
			),
		},
	}

}
