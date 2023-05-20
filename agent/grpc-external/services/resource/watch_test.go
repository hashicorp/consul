// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWatchList_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	testCases := map[string]func(*pbresource.WatchListRequest){
		"no type":    func(req *pbresource.WatchListRequest) { req.Type = nil },
		"no tenancy": func(req *pbresource.WatchListRequest) { req.Tenancy = nil },
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			req := &pbresource.WatchListRequest{
				Type:    demo.TypeV2Album,
				Tenancy: demo.TenancyDefault,
			}
			modFn(req)

			stream, err := client.WatchList(testContext(t), req)
			require.NoError(t, err)

			_, err = stream.Recv()
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		})
	}
}

func TestWatchList_TypeNotFound(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	client := testClient(t, server)

	stream, err := client.WatchList(context.Background(), &pbresource.WatchListRequest{
		Type:       demo.TypeV2Artist,
		Tenancy:    demo.TenancyDefault,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	err = mustGetError(t, rspCh)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.artist not registered")
}

func TestWatchList_GroupVersionMatches(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)
	ctx := context.Background()

	// create a watch
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       demo.TypeV2Artist,
		Tenancy:    demo.TenancyDefault,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	// insert and verify upsert event received
	r1, err := server.Backend.WriteCAS(ctx, artist)
	require.NoError(t, err)
	rsp := mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, rsp.Operation)
	prototest.AssertDeepEqual(t, r1, rsp.Resource)

	// update and verify upsert event received
	r2 := modifyArtist(t, r1)
	r2, err = server.Backend.WriteCAS(ctx, r2)
	require.NoError(t, err)
	rsp = mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, rsp.Operation)
	prototest.AssertDeepEqual(t, r2, rsp.Resource)

	// delete and verify delete event received
	err = server.Backend.DeleteCAS(ctx, r2.Id, r2.Version)
	require.NoError(t, err)
	rsp = mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchEvent_OPERATION_DELETE, rsp.Operation)
}

func TestWatchList_GroupVersionMismatch(t *testing.T) {
	// Given a watch on TypeArtistV1 that only differs from TypeArtistV2 by GroupVersion
	// When a resource of TypeArtistV2 is created/updated/deleted
	// Then no watch events should be emitted
	t.Parallel()

	server := testServer(t)
	demo.RegisterTypes(server.Registry)
	client := testClient(t, server)
	ctx := context.Background()

	// create a watch for TypeArtistV1
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       demo.TypeV1Artist,
		Tenancy:    demo.TenancyDefault,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	// insert
	r1, err := server.Backend.WriteCAS(ctx, artist)
	require.NoError(t, err)

	// update
	r2 := clone(r1)
	r2, err = server.Backend.WriteCAS(ctx, r2)
	require.NoError(t, err)

	// delete
	err = server.Backend.DeleteCAS(ctx, r2.Id, r2.Version)
	require.NoError(t, err)

	// verify no events received
	mustGetNoResource(t, rspCh)
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestWatchList_ACL_ListDenied(t *testing.T) {
	t.Parallel()

	// deny all
	rspCh, _ := roundTripACL(t, testutils.ACLNoPermissions(t))

	// verify key:list denied
	err := mustGetError(t, rspCh)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "lacks permission 'key:list'")
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestWatchList_ACL_ListAllowed_ReadDenied(t *testing.T) {
	t.Parallel()

	// allow list, deny read
	authz := AuthorizerFrom(t, `
		key_prefix "resource/" { policy = "list" }
		key_prefix "resource/demo.v2.artist/" { policy = "deny" }
		`)
	rspCh, _ := roundTripACL(t, authz)

	// verify resource filtered out by key:read denied, hence no events
	mustGetNoResource(t, rspCh)
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestWatchList_ACL_ListAllowed_ReadAllowed(t *testing.T) {
	t.Parallel()

	// allow list, allow read
	authz := AuthorizerFrom(t, `
		key_prefix "resource/" { policy = "list" }
		key_prefix "resource/demo.v2.artist/" { policy = "read" }
	`)
	rspCh, artist := roundTripACL(t, authz)

	// verify resource not filtered out by acl
	event := mustGetResource(t, rspCh)
	prototest.AssertDeepEqual(t, artist, event.Resource)
}

// roundtrip a WatchList which attempts to stream back a single write event
func roundTripACL(t *testing.T, authz acl.Authorizer) (<-chan resourceOrError, *pbresource.Resource) {
	server := testServer(t)
	client := testClient(t, server)

	mockACLResolver := &MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(authz, nil)
	server.ACLResolver = mockACLResolver
	demo.RegisterTypes(server.Registry)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	stream, err := client.WatchList(testContext(t), &pbresource.WatchListRequest{
		Type:       artist.Id.Type,
		Tenancy:    artist.Id.Tenancy,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	// induce single watch event
	artist, err = server.Backend.WriteCAS(context.Background(), artist)
	require.NoError(t, err)

	// caller to make assertions on the rspCh and written artist
	return rspCh, artist
}

func mustGetNoResource(t *testing.T, ch <-chan resourceOrError) {
	t.Helper()

	select {
	case rsp := <-ch:
		require.NoError(t, rsp.err)
		require.Nil(t, rsp.rsp, "expected nil response with no error")
	case <-time.After(250 * time.Millisecond):
		return
	}
}

func mustGetResource(t *testing.T, ch <-chan resourceOrError) *pbresource.WatchEvent {
	t.Helper()

	select {
	case rsp := <-ch:
		require.NoError(t, rsp.err)
		return rsp.rsp
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for WatchListResponse")
		return nil
	}
}

func mustGetError(t *testing.T, ch <-chan resourceOrError) error {
	t.Helper()

	select {
	case rsp := <-ch:
		require.Error(t, rsp.err)
		return rsp.err
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for WatchListResponse")
		return nil
	}
}

func handleResourceStream(t *testing.T, stream pbresource.ResourceService_WatchListClient) <-chan resourceOrError {
	t.Helper()

	rspCh := make(chan resourceOrError)
	go func() {
		for {
			rsp, err := stream.Recv()
			if errors.Is(err, io.EOF) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return
			}
			rspCh <- resourceOrError{
				rsp: rsp,
				err: err,
			}
		}
	}()
	return rspCh
}

type resourceOrError struct {
	rsp *pbresource.WatchEvent
	err error
}
