package resource

import (
	context "context"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	storage "github.com/hashicorp/consul/internal/storage"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
			mockBackend := &MockBackend{}
			mockBackend.On("Read", mock.Anything, mock.Anything, mock.Anything).Return(nil, storage.ErrNotFound)

			server := NewServer(Config{
				registry: resource.NewRegistry(),
				backend:  mockBackend,
			})
			server.registry.Register(resource.Registration{Type: typev1})
			client := testClient(t, server)

			_, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: id1})
			require.Error(t, err)
			require.Equal(t, codes.NotFound.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource not found")
			mockBackend.AssertCalled(t, "Read", mock.Anything, tc.consistency, mock.Anything)
		})
	}
}

func TestRead_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			mockBackend := &MockBackend{}
			mockBackend.On("Read", mock.Anything, mock.Anything, mock.Anything).Return(nil, storage.GroupVersionMismatchError{
				RequestedType: &pbresource.Type{GroupVersion: "v2"},
				Stored:        &pbresource.Resource{Id: &pbresource.ID{Type: &pbresource.Type{GroupVersion: "v1"}}},
			})

			server := NewServer(Config{
				registry: resource.NewRegistry(),
				backend:  mockBackend,
			})
			server.registry.Register(resource.Registration{Type: typev1})
			server.registry.Register(resource.Registration{Type: typev2})
			client := testClient(t, server)

			_, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: id2})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource was requested with GroupVersion")
			mockBackend.AssertCalled(t, "Read", mock.Anything, tc.consistency, mock.Anything)
		})
	}
}

func TestRead_Success(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			resource1 := &pbresource.Resource{Id: id1}
			mockBackend := &MockBackend{}
			mockBackend.On("Read", mock.Anything, mock.Anything, mock.Anything).Return(resource1, nil)

			server := NewServer(Config{
				registry: resource.NewRegistry(),
				backend:  mockBackend,
			})
			server.registry.Register(resource.Registration{Type: typev1})
			client := testClient(t, server)

			_, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: id1})
			require.NoError(t, err)
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

var (
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
	tenancy = &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
		PeerName:  "local",
	}
	id1 = &pbresource.ID{
		Uid:  "abcd",
		Name: "billing",
		Type: typev1,
	}
	id2 = &pbresource.ID{
		Uid:  "abcd",
		Name: "billing",
		Type: typev2,
	}
)
