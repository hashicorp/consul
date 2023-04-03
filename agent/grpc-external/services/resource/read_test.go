package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestRead_TypeNotFound(t *testing.T) {
	server := NewServer(Config{Registry: resource.NewRegistry()})
	client := testClient(t, server)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Read(context.Background(), &pbresource.ReadRequest{Id: artist.Id})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo/v2/artist not registered")
}

func TestRead_ResourceNotFound(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)

			demo.Register(server.Registry, nil)
			client := testClient(t, server)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			_, err = client.Read(tc.ctx, &pbresource.ReadRequest{Id: artist.Id})
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

			demo.Register(server.Registry, nil)
			client := testClient(t, server)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			_, err = server.Backend.WriteCAS(tc.ctx, artist)
			require.NoError(t, err)

			id := clone(artist.Id)
			id.Type = demo.TypeV1Artist

			_, err = client.Read(tc.ctx, &pbresource.ReadRequest{Id: id})
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

			demo.Register(server.Registry, nil)
			client := testClient(t, server)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			resource1, err := server.Backend.WriteCAS(tc.ctx, artist)
			require.NoError(t, err)

			rsp, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: artist.Id})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, resource1, rsp.Resource)
		})
	}
}

func TestRead_VerifyReadConsistencyArg(t *testing.T) {
	// Uses a mockBackend instead of the inmem Backend to verify the ReadConsistency argument is set correctly.
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			mockBackend := NewMockBackend(t)
			server.Backend = mockBackend
			demo.Register(server.Registry, nil)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			mockBackend.On("Read", mock.Anything, mock.Anything, mock.Anything).Return(artist, nil)
			client := testClient(t, server)

			rsp, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: artist.Id})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, artist, rsp.Resource)
			mockBackend.AssertCalled(t, "Read", mock.Anything, tc.consistency, mock.Anything)
		})
	}
}

func TestRead_ACL_RegisteredHook(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	called := false
	demo.Register(server.Registry, &resource.ACLHooks{
		Read: func(authz acl.Authorizer, id *pbresource.ID) error {
			called = true
			return acl.ErrPermissionDenied
		},
	})
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Read(testContext(t), &pbresource.ReadRequest{Id: artist.Id})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	require.True(t, called)
}

func TestRead_ACL_DefaultHook_PermissionDenied(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	mockACLResolver := &MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLNoPermissions(t), nil)
	server.ACLResolver = mockACLResolver

	demo.Register(server.Registry, nil)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Read(testContext(t), &pbresource.ReadRequest{Id: artist.Id})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
}

func TestRead_ACL_DefaultHook_PermissionGranted(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	mockACLResolver := &MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLOperatorRead(t), nil)
	server.ACLResolver = mockACLResolver

	demo.Register(server.Registry, nil)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	resource1, err := server.Backend.WriteCAS(context.Background(), artist)
	require.NoError(t, err)

	rsp, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: artist.Id})
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, resource1, rsp.Resource)
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
