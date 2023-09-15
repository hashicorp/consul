// // Copyright (c) HashiCorp, Inc.
// // SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestListByOwner_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)

	testCases := map[string]func(artistId, recordlabelId *pbresource.ID) *pbresource.ID{
		"no owner": func(artistId, recordLabelId *pbresource.ID) *pbresource.ID {
			return nil
		},
		"no type": func(artistId, _ *pbresource.ID) *pbresource.ID {
			artistId.Type = nil
			return artistId
		},
		"no name": func(artistId, _ *pbresource.ID) *pbresource.ID {
			artistId.Name = ""
			return artistId
		},
		"no uid": func(artistId, _ *pbresource.ID) *pbresource.ID {
			artistId.Uid = ""
			return artistId
		},
		"partition scope with non-empty namespace": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
			recordLabelId.Tenancy.Namespace = "ishouldnothaveanamespace"
			return recordLabelId
		},
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
			require.NoError(t, err)

			// Each test case picks which resource to use based on the resource type's scope.
			req := &pbresource.ListByOwnerRequest{Owner: modFn(artist.Id, recordLabel.Id)}

			_, err = client.ListByOwner(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		})
	}
}

func TestListByOwner_TypeNotRegistered(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	_, err := client.ListByOwner(context.Background(), &pbresource.ListByOwnerRequest{
		Owner: &pbresource.ID{
			Type:    demo.TypeV2Artist,
			Tenancy: resource.DefaultNamespacedTenancy(),
			Uid:     "bogus",
			Name:    "bogus",
		},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.Artist not registered")
}

func TestListByOwner_Empty(t *testing.T) {
	server := testServer(t)
	demo.RegisterTypes(server.Registry)
	client := testClient(t, server)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	rsp2, err := client.ListByOwner(testContext(t), &pbresource.ListByOwnerRequest{Owner: rsp1.Resource.Id})
	require.NoError(t, err)
	require.Empty(t, rsp2.Resources)
}

func TestListByOwner_Many(t *testing.T) {
	server := testServer(t)
	demo.RegisterTypes(server.Registry)
	client := testClient(t, server)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	artist := rsp1.Resource
	require.NoError(t, err)

	albums := make([]*pbresource.Resource, 10)
	for i := 0; i < len(albums); i++ {
		album, err := demo.GenerateV2Album(artist.Id)
		require.NoError(t, err)

		// Prevent test flakes if the generated names collide.
		album.Id.Name = fmt.Sprintf("%s-%d", artist.Id.Name, i)

		rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
		require.NoError(t, err)
		albums[i] = rsp2.Resource
	}

	rsp3, err := client.ListByOwner(testContext(t), &pbresource.ListByOwnerRequest{
		Owner: artist.Id,
	})
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, albums, rsp3.Resources)
}

func TestListByOwner_OwnerTenancyDoesNotExist(t *testing.T) {
	tenancyCases := map[string]func(artistId, recordlabelId *pbresource.ID) *pbresource.ID{
		"partition not found when namespace scoped": func(artistId, _ *pbresource.ID) *pbresource.ID {
			id := clone(artistId)
			id.Uid = "doesnotmatter"
			id.Tenancy.Partition = "boguspartition"
			return id
		},
		"namespace not found when namespace scoped": func(artistId, _ *pbresource.ID) *pbresource.ID {
			id := clone(artistId)
			id.Uid = "doesnotmatter"
			id.Tenancy.Namespace = "bogusnamespace"
			return id
		},
		"partition not found when partition scoped": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
			id := clone(recordLabelId)
			id.Uid = "doesnotmatter"
			id.Tenancy.Partition = "boguspartition"
			return id
		},
	}
	for desc, modFn := range tenancyCases {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			demo.RegisterTypes(server.Registry)
			client := testClient(t, server)

			recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
			require.NoError(t, err)
			recordLabel, err = server.Backend.WriteCAS(testContext(t), recordLabel)
			require.NoError(t, err)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)
			artist, err = server.Backend.WriteCAS(testContext(t), artist)
			require.NoError(t, err)

			// Verify non-existant tenancy units in owner err with not found.
			_, err = client.ListByOwner(testContext(t), &pbresource.ListByOwnerRequest{Owner: modFn(artist.Id, recordLabel.Id)})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource not found")
		})
	}
}

func TestListByOwner_Tenancy_Defaults_And_Normalization(t *testing.T) {
	for tenancyDesc, modFn := range tenancyCases() {
		t.Run(tenancyDesc, func(t *testing.T) {
			server := testServer(t)
			demo.RegisterTypes(server.Registry)
			client := testClient(t, server)

			// Create partition scoped recordLabel.
			recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
			require.NoError(t, err)
			rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: recordLabel})
			require.NoError(t, err)
			recordLabel = rsp1.Resource

			// Create namespace scoped artist.
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)
			rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
			require.NoError(t, err)
			artist = rsp2.Resource

			// Owner will be either partition scoped (recordLabel) or namespace scoped (artist) based on testcase.
			moddedOwnerId := modFn(artist.Id, recordLabel.Id)
			var ownerId *pbresource.ID

			// Avoid using the modded id when linking owner to child.
			switch {
			case proto.Equal(moddedOwnerId.Type, demo.TypeV2Artist):
				ownerId = artist.Id
			case proto.Equal(moddedOwnerId.Type, demo.TypeV1RecordLabel):
				ownerId = recordLabel.Id
			default:
				require.Fail(t, "unexpected resource type")
			}

			// Link owner to child.
			album, err := demo.GenerateV2Album(ownerId)
			require.NoError(t, err)
			rsp3, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
			require.NoError(t, err)
			album = rsp3.Resource

			// Test
			listRsp, err := client.ListByOwner(testContext(t), &pbresource.ListByOwnerRequest{
				Owner: moddedOwnerId,
			})
			require.NoError(t, err)

			// Verify child album always returned.
			prototest.AssertDeepEqual(t, album, listRsp.Resources[0])
		})
	}
}

func TestListByOwner_ACL_PerTypeDenied(t *testing.T) {
	authz := AuthorizerFrom(t, `key_prefix "resource/demo.v2.Album/" { policy = "deny" }`, demo.ArtistV2ListPolicy)
	_, rsp, err := roundTripListByOwner(t, authz)

	// verify resource filtered out, hence no results
	require.NoError(t, err)
	require.Empty(t, rsp.Resources)
}

func TestListByOwner_ACL_PerTypeAllowed(t *testing.T) {
	authz := AuthorizerFrom(t, `key_prefix "resource/demo.v2.Album/" { policy = "read" }`, demo.ArtistV2ListPolicy)
	album, rsp, err := roundTripListByOwner(t, authz)

	// verify resource not filtered out
	require.NoError(t, err)
	require.Len(t, rsp.Resources, 1)
	prototest.AssertDeepEqual(t, album, rsp.Resources[0])
}

// roundtrip a ListByOwner which attempts to return a single resource
func roundTripListByOwner(t *testing.T, authz acl.Authorizer) (*pbresource.Resource, *pbresource.ListByOwnerResponse, error) {
	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
	artist = rsp1.Resource
	require.NoError(t, err)

	album, err := demo.GenerateV2Album(artist.Id)
	require.NoError(t, err)

	rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
	album = rsp2.Resource
	require.NoError(t, err)

	mockACLResolver := &MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(authz, nil)
	server.ACLResolver = mockACLResolver

	rsp3, err := client.ListByOwner(testContext(t), &pbresource.ListByOwnerRequest{Owner: artist.Id})
	return album, rsp3, err
}
