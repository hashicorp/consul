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
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
	demo.Register(server.Registry, nil)
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
	demo.Register(server.Registry, nil)
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

func TestWatchList_ACL_RegisteredHooks(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	client := testClient(t, server)
	ctx := context.Background()

	readCalled := false
	listCalled := false
	demo.Register(server.Registry, &resource.ACLHooks{
		Read: func(authz acl.Authorizer, id *pbresource.ID) error {
			readCalled = true
			return nil
		},
		List: func(authz acl.Authorizer, tenancy *pbresource.Tenancy) error {
			listCalled = true
			return nil
		},
	})

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

	// induce single watch event
	_, err = server.Backend.WriteCAS(ctx, artist)
	require.NoError(t, err)
	_ = mustGetResource(t, rspCh)

	// verify both registered ACL hooks called
	require.True(t, listCalled)
	require.True(t, readCalled)
}

func TestWatchList_ACL_DefaultListHook_PermissionDenied(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	client := testClient(t, server)
	ctx := context.Background()

	mockACLResolver := &MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLNoPermissions(t), nil)
	server.ACLResolver = mockACLResolver

	// There two two hooks in play: List and Read. To make sure only the default List hook is
	// exercised by this test and is the origin of PermissionDenied, provide a Read hook
	// and verify it is never called.
	readCalled := false
	demo.Register(server.Registry, &resource.ACLHooks{
		Read: func(authz acl.Authorizer, id *pbresource.ID) error {
			readCalled = true
			return nil
		},
	})

	// create a watch
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       demo.TypeV2Artist,
		Tenancy:    demo.TenancyDefault,
		NamePrefix: "",
	})
	require.NoError(t, err)
	require.False(t, readCalled)
	rspCh := handleResourceStream(t, stream)

	// verify persmission denied
	err = mustGetError(t, rspCh)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	require.False(t, readCalled)
}

func TestWatchList_ACL_DefaultReadHook_PermissionDenied(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	ctx := context.Background()

	mockACLResolver := &MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLNoPermissions(t), nil)
	server.ACLResolver = mockACLResolver

	// Allow List ACL check to pass so Read ACL check can be reached
	listCalled := false
	demo.Register(server.Registry, &resource.ACLHooks{
		List: func(authz acl.Authorizer, tenancy *pbresource.Tenancy) error {
			listCalled = true
			return nil
		},
	})
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	_, err = server.Backend.WriteCAS(context.Background(), artist)
	require.NoError(t, err)

	// create a watch
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       demo.TypeV2Artist,
		Tenancy:    demo.TenancyDefault,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	// verify no resource since ACL filtered out event
	mustGetNoResource(t, rspCh)
	require.True(t, listCalled)
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
