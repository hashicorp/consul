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
	t.Parallel()

	_, client, ctx := testDeps(t)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	// delete artist with unregistered type
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: artist.Id, Version: ""})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
}

func TestDelete_Success(t *testing.T) {
	t.Parallel()

	for desc, tc := range deleteTestCases() {
		t.Run(desc, func(t *testing.T) {
			server, client, ctx := testDeps(t)
			demo.Register(server.Registry)
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)
			rsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
			require.NoError(t, err)

			// delete
			_, err = client.Delete(ctx, &pbresource.DeleteRequest{
				Id:      rsp.Resource.Id,
				Version: tc.versionFn(rsp.Resource),
			})
			require.NoError(t, err)

			// verify deleted
			_, err = server.Backend.Read(ctx, storage.StrongConsistency, rsp.Resource.Id)
			require.Error(t, err)
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	}
}

func TestDelete_NotFound(t *testing.T) {
	t.Parallel()

	for desc, tc := range deleteTestCases() {
		t.Run(desc, func(t *testing.T) {
			server, client, ctx := testDeps(t)
			demo.Register(server.Registry)
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			// verify delete of non-existant or already deleted resource is a no-op
			_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: artist.Id, Version: tc.versionFn(artist)})
			require.NoError(t, err)
		})
	}
}

func TestDelete_VersionMismatch(t *testing.T) {
	t.Parallel()

	server, client, ctx := testDeps(t)
	demo.Register(server.Registry)
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	rsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)

	// delete with a version that is different from the stored version
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: rsp.Resource.Id, Version: "non-existent-version"})
	require.Error(t, err)
	require.Equal(t, codes.Aborted.String(), status.Code(err).String())
	require.ErrorContains(t, err, "CAS operation failed")
}

func testDeps(t *testing.T) (*Server, pbresource.ResourceServiceClient, context.Context) {
	server := testServer(t)
	client := testClient(t, server)
	return server, client, context.Background()
}

type deleteTestCase struct {
	// returns the version to use in the test given the passed in resource
	versionFn func(*pbresource.Resource) string
}

func deleteTestCases() map[string]deleteTestCase {
	return map[string]deleteTestCase{
		"specific version": {
			versionFn: func(r *pbresource.Resource) string {
				return r.Version
			},
		},
		"empty version": {
			versionFn: func(r *pbresource.Resource) string {
				return ""
			},
		},
	}
}
