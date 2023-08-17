// // Copyright (c) HashiCorp, Inc.
// // SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestListByOwner_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	testCases := map[string]func(*pbresource.ListByOwnerRequest){
		"no owner":   func(req *pbresource.ListByOwnerRequest) { req.Owner = nil },
		"no type":    func(req *pbresource.ListByOwnerRequest) { req.Owner.Type = nil },
		"no tenancy": func(req *pbresource.ListByOwnerRequest) { req.Owner.Tenancy = nil },
		"no name":    func(req *pbresource.ListByOwnerRequest) { req.Owner.Name = "" },
		"no uid":     func(req *pbresource.ListByOwnerRequest) { req.Owner.Uid = "" },
		// clone necessary to not pollute DefaultTenancy
		"tenancy partition not default": func(req *pbresource.ListByOwnerRequest) {
			req.Owner.Tenancy = clone(req.Owner.Tenancy)
			req.Owner.Tenancy.Partition = ""
		},
		"tenancy namespace not default": func(req *pbresource.ListByOwnerRequest) {
			req.Owner.Tenancy = clone(req.Owner.Tenancy)
			req.Owner.Tenancy.Namespace = ""
		},
		"tenancy peername not local": func(req *pbresource.ListByOwnerRequest) {
			req.Owner.Tenancy = clone(req.Owner.Tenancy)
			req.Owner.Tenancy.PeerName = ""
		},
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			res, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			req := &pbresource.ListByOwnerRequest{Owner: res.Id}
			modFn(req)

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
			Tenancy: demo.TenancyDefault,
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

func TestListByOwner_ACL_PerTypeDenied(t *testing.T) {
	authz := AuthorizerFrom(t, `key_prefix "resource/demo.v2.Album/" { policy = "deny" }`)
	_, rsp, err := roundTripListByOwner(t, authz)

	// verify resource filtered out, hence no results
	require.NoError(t, err)
	require.Empty(t, rsp.Resources)
}

func TestListByOwner_ACL_PerTypeAllowed(t *testing.T) {
	authz := AuthorizerFrom(t, `key_prefix "resource/demo.v2.Album/" { policy = "read" }`)
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
