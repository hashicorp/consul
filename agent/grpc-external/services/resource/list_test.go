// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestList_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	testCases := map[string]func(*pbresource.ListRequest){
		"no type":    func(req *pbresource.ListRequest) { req.Type = nil },
		"no tenancy": func(req *pbresource.ListRequest) { req.Tenancy = nil },
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			req := &pbresource.ListRequest{
				Type:    demo.TypeV2Album,
				Tenancy: demo.TenancyDefault,
			}
			modFn(req)

			_, err := client.List(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		})
	}
}

func TestList_TypeNotFound(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	_, err := client.List(context.Background(), &pbresource.ListRequest{
		Type:       demo.TypeV2Artist,
		Tenancy:    demo.TenancyDefault,
		NamePrefix: "",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.artist not registered")
}

func TestList_Empty(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			demo.RegisterTypes(server.Registry)
			client := testClient(t, server)

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       demo.TypeV1Artist,
				Tenancy:    demo.TenancyDefault,
				NamePrefix: "",
			})
			require.NoError(t, err)
			require.Empty(t, rsp.Resources)
		})
	}
}

func TestList_Many(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			demo.RegisterTypes(server.Registry)
			client := testClient(t, server)

			resources := make([]*pbresource.Resource, 10)
			for i := 0; i < len(resources); i++ {
				artist, err := demo.GenerateV2Artist()
				require.NoError(t, err)

				// Prevent test flakes if the generated names collide.
				artist.Id.Name = fmt.Sprintf("%s-%d", artist.Id.Name, i)
				_, err = server.Backend.WriteCAS(tc.ctx, artist)
				require.NoError(t, err)

				resources[i] = artist
			}

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       demo.TypeV2Artist,
				Tenancy:    demo.TenancyDefault,
				NamePrefix: "",
			})
			require.NoError(t, err)
			prototest.AssertElementsMatch(t, resources, rsp.Resources)
		})
	}
}

func TestList_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			demo.RegisterTypes(server.Registry)
			client := testClient(t, server)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			_, err = server.Backend.WriteCAS(tc.ctx, artist)
			require.NoError(t, err)

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       demo.TypeV1Artist,
				Tenancy:    artist.Id.Tenancy,
				NamePrefix: "",
			})
			require.NoError(t, err)
			require.Empty(t, rsp.Resources)
		})
	}
}

func TestList_VerifyReadConsistencyArg(t *testing.T) {
	// Uses a mockBackend instead of the inmem Backend to verify the ReadConsistency argument is set correctly.
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			mockBackend := NewMockBackend(t)
			server := testServer(t)
			server.Backend = mockBackend
			demo.RegisterTypes(server.Registry)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			mockBackend.On("List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]*pbresource.Resource{artist}, nil)
			client := testClient(t, server)

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{Type: artist.Id.Type, Tenancy: artist.Id.Tenancy, NamePrefix: ""})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, artist, rsp.Resources[0])
			mockBackend.AssertCalled(t, "List", mock.Anything, tc.consistency, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestList_ACL_ListDenied(t *testing.T) {
	t.Parallel()

	// deny all
	_, _, err := roundTripList(t, testutils.ACLNoPermissions(t))

	// verify key:list denied
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "lacks permission 'key:list'")
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestList_ACL_ListAllowed_ReadDenied(t *testing.T) {
	t.Parallel()

	// allow list, deny read
	authz := AuthorizerFrom(t, demo.ArtistV2ListPolicy,
		`key_prefix "resource/demo.v2.artist/" { policy = "deny" }`)
	_, rsp, err := roundTripList(t, authz)

	// verify resource filtered out by key:read denied hence no results
	require.NoError(t, err)
	require.Empty(t, rsp.Resources)
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestList_ACL_ListAllowed_ReadAllowed(t *testing.T) {
	t.Parallel()

	// allow list, allow read
	authz := AuthorizerFrom(t, demo.ArtistV2ListPolicy, demo.ArtistV2ReadPolicy)
	artist, rsp, err := roundTripList(t, authz)

	// verify resource not filtered out by acl
	require.NoError(t, err)
	require.Len(t, rsp.Resources, 1)
	prototest.AssertDeepEqual(t, artist, rsp.Resources[0])
}

// roundtrip a List which attempts to return a single resource
func roundTripList(t *testing.T, authz acl.Authorizer) (*pbresource.Resource, *pbresource.ListResponse, error) {
	server := testServer(t)
	client := testClient(t, server)
	ctx := testContext(t)

	mockACLResolver := &MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(authz, nil)
	server.ACLResolver = mockACLResolver
	demo.RegisterTypes(server.Registry)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	artist, err = server.Backend.WriteCAS(ctx, artist)
	require.NoError(t, err)

	rsp, err := client.List(
		ctx,
		&pbresource.ListRequest{
			Type:       artist.Id.Type,
			Tenancy:    artist.Id.Tenancy,
			NamePrefix: "",
		},
	)

	return artist, rsp, err
}

type listTestCase struct {
	consistency storage.ReadConsistency
	ctx         context.Context
}

func listTestCases() map[string]listTestCase {
	return map[string]listTestCase{
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
