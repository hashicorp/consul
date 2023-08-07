// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestDelete_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	testCases := map[string]func(*pbresource.DeleteRequest){
		"no id":      func(req *pbresource.DeleteRequest) { req.Id = nil },
		"no type":    func(req *pbresource.DeleteRequest) { req.Id.Type = nil },
		"no tenancy": func(req *pbresource.DeleteRequest) { req.Id.Tenancy = nil },
		"no name":    func(req *pbresource.DeleteRequest) { req.Id.Name = "" },

		// TODO(spatel): Refactor tenancy as part of NET-4919
		//
		// clone necessary to not pollute DefaultTenancy
		// "tenancy partition not default": func(req *pbresource.DeleteRequest) {
		// 	req.Id.Tenancy = clone(req.Id.Tenancy)
		// 	req.Id.Tenancy.Partition = ""
		// },
		// "tenancy namespace not default": func(req *pbresource.DeleteRequest) {
		// 	req.Id.Tenancy = clone(req.Id.Tenancy)
		// 	req.Id.Tenancy.Namespace = ""
		// },
		// "tenancy peername not local": func(req *pbresource.DeleteRequest) {
		// 	req.Id.Tenancy = clone(req.Id.Tenancy)
		// 	req.Id.Tenancy.PeerName = ""
		// },
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			res, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			req := &pbresource.DeleteRequest{Id: res.Id, Version: ""}
			modFn(req)

			_, err = client.Delete(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		})
	}
}

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

func TestDelete_ACLs(t *testing.T) {
	type testCase struct {
		authz       resolver.Result
		assertErrFn func(error)
	}
	testcases := map[string]testCase{
		"delete denied": {
			authz: AuthorizerFrom(t, demo.ArtistV1WritePolicy),
			assertErrFn: func(err error) {
				require.Error(t, err)
				require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
			},
		},
		"delete allowed": {
			authz: AuthorizerFrom(t, demo.ArtistV2WritePolicy),
			assertErrFn: func(err error) {
				require.NoError(t, err)
			},
		},
	}

	for desc, tc := range testcases {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			client := testClient(t, server)

			mockACLResolver := &MockACLResolver{}
			mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
				Return(tc.authz, nil)
			server.ACLResolver = mockACLResolver
			demo.RegisterTypes(server.Registry)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			artist, err = server.Backend.WriteCAS(context.Background(), artist)
			require.NoError(t, err)

			// exercise ACL
			_, err = client.Delete(testContext(t), &pbresource.DeleteRequest{Id: artist.Id})
			tc.assertErrFn(err)
		})
	}
}

func TestDelete_Success(t *testing.T) {
	t.Parallel()

	for desc, tc := range deleteTestCases() {
		t.Run(desc, func(t *testing.T) {
			server, client, ctx := testDeps(t)
			demo.RegisterTypes(server.Registry)
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			rsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
			require.NoError(t, err)
			artistId := clone(rsp.Resource.Id)
			artist = rsp.Resource

			// delete
			_, err = client.Delete(ctx, tc.deleteReqFn(artist))
			require.NoError(t, err)

			// verify deleted
			_, err = server.Backend.Read(ctx, storage.StrongConsistency, artistId)
			require.Error(t, err)
			require.ErrorIs(t, err, storage.ErrNotFound)

			// verify tombstone created
			_, err = client.Read(ctx, &pbresource.ReadRequest{
				Id: &pbresource.ID{
					Name:    tombstoneName(artistId),
					Type:    resource.TypeV1Tombstone,
					Tenancy: artist.Id.Tenancy,
				},
			})
			require.NoError(t, err)
		})
	}
}

func TestDelete_TombstoneDeletionDoesNotCreateNewTombstone(t *testing.T) {
	t.Parallel()

	server, client, ctx := testDeps(t)
	demo.RegisterTypes(server.Registry)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)
	artist = rsp.Resource

	// delete artist
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: artist.Id, Version: ""})
	require.NoError(t, err)

	// verify artist's tombstone created
	rsp2, err := client.Read(ctx, &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Name:    tombstoneName(artist.Id),
			Type:    resource.TypeV1Tombstone,
			Tenancy: artist.Id.Tenancy,
		},
	})
	require.NoError(t, err)
	tombstone := rsp2.Resource

	// delete artist's tombstone
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: tombstone.Id, Version: tombstone.Version})
	require.NoError(t, err)

	// verify no new tombstones created and artist's existing tombstone deleted
	rsp3, err := client.List(ctx, &pbresource.ListRequest{Type: resource.TypeV1Tombstone, Tenancy: artist.Id.Tenancy})
	require.NoError(t, err)
	require.Empty(t, rsp3.Resources)
}

func TestDelete_NotFound(t *testing.T) {
	t.Parallel()

	for desc, tc := range deleteTestCases() {
		t.Run(desc, func(t *testing.T) {
			server, client, ctx := testDeps(t)
			demo.RegisterTypes(server.Registry)
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			// verify delete of non-existant or already deleted resource is a no-op
			_, err = client.Delete(ctx, tc.deleteReqFn(artist))
			require.NoError(t, err)
		})
	}
}

func TestDelete_VersionMismatch(t *testing.T) {
	t.Parallel()

	server, client, ctx := testDeps(t)
	demo.RegisterTypes(server.Registry)
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
	deleteReqFn func(r *pbresource.Resource) *pbresource.DeleteRequest
}

func deleteTestCases() map[string]deleteTestCase {
	return map[string]deleteTestCase{
		"version and uid": {
			deleteReqFn: func(r *pbresource.Resource) *pbresource.DeleteRequest {
				return &pbresource.DeleteRequest{Id: r.Id, Version: r.Version}
			},
		},
		"version only": {
			deleteReqFn: func(r *pbresource.Resource) *pbresource.DeleteRequest {
				r.Id.Uid = ""
				return &pbresource.DeleteRequest{Id: r.Id, Version: r.Version}
			},
		},
		"uid only": {
			deleteReqFn: func(r *pbresource.Resource) *pbresource.DeleteRequest {
				return &pbresource.DeleteRequest{Id: r.Id, Version: ""}
			},
		},
		"no version or uid": {
			deleteReqFn: func(r *pbresource.Resource) *pbresource.DeleteRequest {
				r.Id.Uid = ""
				return &pbresource.DeleteRequest{Id: r.Id, Version: ""}
			},
		},
	}
}
