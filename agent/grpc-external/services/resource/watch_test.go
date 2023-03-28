// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestWatchList_TypeNotFound(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	client := testClient(t, server)

	stream, err := client.WatchList(context.Background(), &pbresource.WatchListRequest{
		Type:       typev1,
		Tenancy:    tenancy,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	err = mustGetError(t, rspCh)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type mesh/v1/service not registered")
}

func TestWatchList_GroupVersionMatches(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	client := testClient(t, server)
	server.registry.Register(resource.Registration{Type: typev1})
	ctx := context.Background()

	// create a watch
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       typev1,
		Tenancy:    tenancy,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	// insert and verify upsert event received
	r1, err := server.backend.WriteCAS(ctx, resourcev1)
	require.NoError(t, err)
	rsp := mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, rsp.Operation)
	prototest.AssertDeepEqual(t, r1, rsp.Resource)

	// update and verify upsert event received
	r2 := clone(r1)
	r2, err = server.backend.WriteCAS(ctx, r2)
	require.NoError(t, err)
	rsp = mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, rsp.Operation)
	prototest.AssertDeepEqual(t, r2, rsp.Resource)

	// delete and verify delete event received
	err = server.backend.DeleteCAS(ctx, r2.Id, r2.Version)
	require.NoError(t, err)
	rsp = mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchEvent_OPERATION_DELETE, rsp.Operation)
}

func TestWatchList_GroupVersionMismatch(t *testing.T) {
	// Given a watch on typev2 that only differs from typev1 by GroupVersion
	// When a resource of typev1 is created/updated/deleted
	// Then no watch events should be emitted
	t.Parallel()
	server := testServer(t)
	client := testClient(t, server)
	ctx := context.Background()
	server.registry.Register(resource.Registration{Type: typev1})
	server.registry.Register(resource.Registration{Type: typev2})

	// create a watch for typev2
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       typev2,
		Tenancy:    tenancy,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	// insert
	r1, err := server.backend.WriteCAS(ctx, resourcev1)
	require.NoError(t, err)

	// update
	r2 := clone(r1)
	r2, err = server.backend.WriteCAS(ctx, r2)
	require.NoError(t, err)

	// delete
	err = server.backend.DeleteCAS(ctx, r2.Id, r2.Version)
	require.NoError(t, err)

	// verify no events received
	mustGetNoResource(t, rspCh)
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

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
