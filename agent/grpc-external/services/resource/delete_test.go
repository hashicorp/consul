package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestDelete_Success(t *testing.T) {
	server, client, ctx := testDeps(t)
	server.registry.Register(resource.Registration{Type: typev1})
	resource1 := &pbresource.Resource{Id: id1, Version: ""}
	resource1, err := server.backend.WriteCAS(ctx, resource1)
	require.NoError(t, err)

	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: resource1.Id, Version: resource1.Version})
	require.NoError(t, err)

	// verify really deleted
	_, err = server.backend.Read(ctx, storage.StrongConsistency, resource1.Id)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestDelete_VersionMismatch(t *testing.T) {
	server, client, ctx := testDeps(t)
	server.registry.Register(resource.Registration{Type: typev1})
	resource1 := &pbresource.Resource{Id: id1, Version: ""}
	resource1, err := server.backend.WriteCAS(ctx, resource1)
	require.NoError(t, err)

	// delete resource with a non-existent version
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: resource1.Id, Version: "idontexist"})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
}

func TestDelete_TypeNotRegistered(t *testing.T) {
	_, client, ctx := testDeps(t)

	// delete resource with type that hasn't been registered
	_, err := client.Delete(ctx, &pbresource.DeleteRequest{Id: id1, Version: "blah"})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
}

func testDeps(t *testing.T) (*Server, pbresource.ResourceServiceClient, context.Context) {
	server := testServer(t)
	client := testClient(t, server)
	return server, client, context.Background()
}
