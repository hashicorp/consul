package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestDelete_TypeNotRegistered(t *testing.T) {
	_, client, ctx := testDeps(t)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	// delete artist with unregistered type
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: artist.Id, Version: ""})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
}

func TestDelete_ByVersion_Success(t *testing.T) {
	server, client, ctx := testDeps(t)
	demo.Register(server.Registry)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	_ = artist
	rsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)

	// delete artist by version
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: rsp.Resource.Id, Version: rsp.Resource.Version})
	require.NoError(t, err)

	// verify
	_, err = server.Backend.Read(ctx, storage.StrongConsistency, rsp.Resource.Id)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestDelete_ByVersion_Mismatch(t *testing.T) {
	server, client, ctx := testDeps(t)
	demo.Register(server.Registry)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	_ = artist
	rsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)

	// delete artist with a non-existent version
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: rsp.Resource.Id, Version: "non-existent-version"})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
}

func TestDelete_NonCAS_Success(t *testing.T) {
	server, client, ctx := testDeps(t)
	demo.Register(server.Registry)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	rsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)

	// delete artist regardless of the version
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: rsp.Resource.Id, Version: ""})
	require.NoError(t, err)

	// verify
	_, err = server.Backend.Read(ctx, storage.StrongConsistency, rsp.Resource.Id)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestDelete_ByVersion_NonExistantIsNoOp(t *testing.T) {
	// TODO: Not sure what is expected bahavior here
	server, client, ctx := testDeps(t)
	demo.Register(server.Registry)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: artist.Id, Version: "xxx"})
	require.NoError(t, err)
}

func TestDelete_NonCAS_NonExistantIsError(t *testing.T) {
	// TODO: Not sure what is expected bahavior here
	server, client, ctx := testDeps(t)
	demo.Register(server.Registry)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: artist.Id, Version: ""})
	require.Error(t, err)
}

func testDeps(t *testing.T) (*Server, pbresource.ResourceServiceClient, context.Context) {
	server := testServer(t)
	client := testClient(t, server)
	return server, client, context.Background()
}
