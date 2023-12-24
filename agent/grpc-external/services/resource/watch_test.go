// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

// TODO: Update all tests to use true/false table test for v2tenancy

func TestWatchList_InputValidation(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	type testCase struct {
		modFn       func(*pbresource.WatchListRequest)
		errContains string
	}

	testCases := map[string]testCase{
		"no type": {
			modFn:       func(req *pbresource.WatchListRequest) { req.Type = nil },
			errContains: "type is required",
		},
		"partition mixed case": {
			modFn:       func(req *pbresource.WatchListRequest) { req.Tenancy.Partition = "Default" },
			errContains: "tenancy.partition invalid",
		},
		"partition too long": {
			modFn: func(req *pbresource.WatchListRequest) {
				req.Tenancy.Partition = strings.Repeat("p", resource.MaxNameLength+1)
			},
			errContains: "tenancy.partition invalid",
		},
		"namespace mixed case": {
			modFn:       func(req *pbresource.WatchListRequest) { req.Tenancy.Namespace = "Default" },
			errContains: "tenancy.namespace invalid",
		},
		"namespace too long": {
			modFn: func(req *pbresource.WatchListRequest) {
				req.Tenancy.Namespace = strings.Repeat("n", resource.MaxNameLength+1)
			},
			errContains: "tenancy.namespace invalid",
		},
		"name_prefix mixed case": {
			modFn:       func(req *pbresource.WatchListRequest) { req.NamePrefix = "Smashing" },
			errContains: "name_prefix invalid",
		},
		"partitioned type provides non-empty namespace": {
			modFn: func(req *pbresource.WatchListRequest) {
				req.Type = demo.TypeV1RecordLabel
				req.Tenancy.Namespace = "bad"
			},
			errContains: "cannot have a namespace",
		},
		"cluster scope with non-empty partition": {
			modFn: func(req *pbresource.WatchListRequest) {
				req.Type = demo.TypeV1Executive
				req.Tenancy = &pbresource.Tenancy{Partition: "bad"}
			},
			errContains: "cannot have a partition",
		},
		"cluster scope with non-empty namespace": {
			modFn: func(req *pbresource.WatchListRequest) {
				req.Type = demo.TypeV1Executive
				req.Tenancy = &pbresource.Tenancy{Namespace: "bad"}
			},
			errContains: "cannot have a namespace",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			req := &pbresource.WatchListRequest{
				Type:    demo.TypeV2Album,
				Tenancy: resource.DefaultNamespacedTenancy(),
			}
			tc.modFn(req)

			stream, err := client.WatchList(testContext(t), req)
			require.NoError(t, err)

			_, err = stream.Recv()
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestWatchList_TypeNotFound(t *testing.T) {
	t.Parallel()

	client := svctest.NewResourceServiceBuilder().Run(t)

	stream, err := client.WatchList(context.Background(), &pbresource.WatchListRequest{
		Type:       demo.TypeV2Artist,
		Tenancy:    resource.DefaultNamespacedTenancy(),
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	err = mustGetError(t, rspCh)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.Artist not registered")
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
		Tenancy:    resource.DefaultNamespacedTenancy(),
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

func TestWatchList_Tenancy_Defaults_And_Normalization(t *testing.T) {
	// Test units of tenancy get lowercased and defaulted correctly when empty.
	for desc, tc := range wildcardTenancyCases() {
		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()
			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			// Create a watch.
			stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
				Type:       tc.typ,
				Tenancy:    tc.tenancy,
				NamePrefix: "",
			})
			require.NoError(t, err)
			rspCh := handleResourceStream(t, stream)

			// Testcase will pick one of executive, recordLabel or artist based on scope of type.
			recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
			require.NoError(t, err)
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)
			executive, err := demo.GenerateV1Executive("king-arthur", "CEO")
			require.NoError(t, err)

			// Create and verify upsert event received.
			rlRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: recordLabel})
			require.NoError(t, err)
			artistRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
			require.NoError(t, err)
			executiveRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: executive})
			require.NoError(t, err)

			var expected *pbresource.Resource
			switch {
			case resource.EqualType(tc.typ, demo.TypeV1RecordLabel):
				expected = rlRsp.Resource
			case resource.EqualType(tc.typ, demo.TypeV2Artist):
				expected = artistRsp.Resource
			case resource.EqualType(tc.typ, demo.TypeV1Executive):
				expected = executiveRsp.Resource
			default:
				require.Fail(t, "unsupported type", tc.typ)
			}

			rsp := mustGetResource(t, rspCh)
			require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, rsp.Operation)
			prototest.AssertDeepEqual(t, expected, rsp.Resource)
		})
	}
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
		Tenancy:    resource.DefaultNamespacedTenancy(),
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
		key_prefix "resource/demo.v2.Artist/" { policy = "deny" }
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
		key_prefix "resource/demo.v2.Artist/" { policy = "read" }
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

	mockACLResolver := &svc.MockACLResolver{}
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

func TestWatchList_NoTenancy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	// Create a watch.
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type: demo.TypeV1RecordLabel,
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
	require.NoError(t, err)

	// Create and verify upsert event received.
	rsp1, err := client.Write(ctx, &pbresource.WriteRequest{Resource: recordLabel})
	require.NoError(t, err)

	rsp2 := mustGetResource(t, rspCh)

	require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, rsp2.Operation)
	prototest.AssertDeepEqual(t, rsp1.Resource, rsp2.Resource)
}
