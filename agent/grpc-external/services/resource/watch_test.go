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
	"github.com/hashicorp/consul/acl/resolver"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

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

	b := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes)
	client := b.Run(t)

	ctx := context.Background()

	// create a watch
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       demo.TypeV2Artist,
		Tenancy:    resource.DefaultNamespacedTenancy(),
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	mustGetEndOfSnapshot(t, rspCh)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	// insert and verify upsert event received
	r1Resp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
	r1 := r1Resp.Resource
	require.NoError(t, err)
	rsp := mustGetResource(t, rspCh)
	require.NotNil(t, rsp.GetUpsert())
	prototest.AssertDeepEqual(t, r1, rsp.GetUpsert().Resource)

	// update and verify upsert event received
	r2 := modifyArtist(t, r1)
	r2Resp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: r2})
	require.NoError(t, err)
	r2 = r2Resp.Resource
	rsp = mustGetResource(t, rspCh)
	require.NotNil(t, rsp.GetUpsert())
	prototest.AssertDeepEqual(t, r2, rsp.GetUpsert().Resource)

	// delete and verify delete event received
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: r2.Id, Version: r2.Version})
	require.NoError(t, err)
	rsp = mustGetResource(t, rspCh)
	require.NotNil(t, rsp.GetDelete())
	prototest.AssertDeepEqual(t, r2.Id, rsp.GetDelete().Resource.Id)
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

			mustGetEndOfSnapshot(t, rspCh)

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
			require.NotNil(t, rsp.GetUpsert())
			prototest.AssertDeepEqual(t, expected, rsp.GetUpsert().Resource)
		})
	}
}

func TestWatchList_GroupVersionMismatch(t *testing.T) {
	// Given a watch on TypeArtistV1 that only differs from TypeArtistV2 by GroupVersion
	// When a resource of TypeArtistV2 is created/updated/deleted
	// Then no watch events should be emitted
	t.Parallel()

	b := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes)
	client := b.Run(t)

	ctx := context.Background()

	// create a watch for TypeArtistV1
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       demo.TypeV1Artist,
		Tenancy:    resource.DefaultNamespacedTenancy(),
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	mustGetEndOfSnapshot(t, rspCh)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	// insert
	r1Resp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)
	r1 := r1Resp.Resource

	// update
	r2 := clone(r1)
	r2Resp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: r2})
	require.NoError(t, err)
	r2 = r2Resp.Resource

	// delete
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: r2.Id, Version: r2.Version})
	require.NoError(t, err)

	// verify no events received
	mustGetNoResource(t, rspCh)
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestWatchList_ACL_ListDenied(t *testing.T) {
	t.Parallel()

	// deny all
	rspCh, _ := roundTripACL(t, testutils.ACLNoPermissions(t), true)

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
	rspCh, _ := roundTripACL(t, authz, false)

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
	rspCh, artist := roundTripACL(t, authz, false)

	// verify resource not filtered out by acl
	event := mustGetResource(t, rspCh)

	require.NotNil(t, event.GetUpsert())
	prototest.AssertDeepEqual(t, artist, event.GetUpsert().Resource)
}

// roundtrip a WatchList which attempts to stream back a single write event
func roundTripACL(t *testing.T, authz acl.Authorizer, expectErr bool) (<-chan resourceOrError, *pbresource.Resource) {
	mockACLResolver := &svc.MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(resolver.Result{Authorizer: authz}, nil)

	b := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		WithACLResolver(mockACLResolver)
	client := b.Run(t)
	server := b.ServiceImpl()

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	stream, err := client.WatchList(testContext(t), &pbresource.WatchListRequest{
		Type:       artist.Id.Type,
		Tenancy:    artist.Id.Tenancy,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	if !expectErr {
		mustGetEndOfSnapshot(t, rspCh)
	}

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

func mustGetEndOfSnapshot(t *testing.T, ch <-chan resourceOrError) {
	event := mustGetResource(t, ch)
	require.NotNil(t, event.GetEndOfSnapshot(), "expected EndOfSnapshot but got got event %T", event.GetEvent())
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

	mustGetEndOfSnapshot(t, rspCh)

	recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
	require.NoError(t, err)

	// Create and verify upsert event received.
	rsp1, err := client.Write(ctx, &pbresource.WriteRequest{Resource: recordLabel})
	require.NoError(t, err)

	rsp2 := mustGetResource(t, rspCh)

	require.NotNil(t, rsp2.GetUpsert())
	prototest.AssertDeepEqual(t, rsp1.Resource, rsp2.GetUpsert().Resource)
}
