// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl/resolver"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
)

func TestDelete_InputValidation(t *testing.T) {
	type testCase struct {
		modFn       func(artistId, recordLabelId, executiveId *pbresource.ID) *pbresource.ID
		errContains string
	}

	run := func(t *testing.T, client pbresource.ResourceServiceClient, tc testCase) {
		executive, err := demo.GenerateV1Executive("marvin", "CEO")
		require.NoError(t, err)

		recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
		require.NoError(t, err)

		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		req := &pbresource.DeleteRequest{Id: tc.modFn(artist.Id, recordLabel.Id, executive.Id), Version: ""}
		_, err = client.Delete(context.Background(), req)
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		require.ErrorContains(t, err, tc.errContains)
	}

	testCases := map[string]testCase{
		"no id": {
			modFn: func(_, _, _ *pbresource.ID) *pbresource.ID {
				return nil
			},
			errContains: "id is required",
		},
		"no type": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Type = nil
				return artistId
			},
			errContains: "id.type is required",
		},
		"no name": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Name = ""
				return artistId
			},
			errContains: "id.name invalid",
		},
		"mixed case name": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Name = "DepecheMode"
				return artistId
			},
			errContains: "id.name invalid",
		},
		"name too long": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Name = strings.Repeat("n", resource.MaxNameLength+1)
				return artistId
			},
			errContains: "id.name invalid",
		},
		"partition mixed case": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Tenancy.Partition = "Default"
				return artistId
			},
			errContains: "id.tenancy.partition invalid",
		},
		"partition name too long": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Tenancy.Partition = strings.Repeat("p", resource.MaxNameLength+1)
				return artistId
			},
			errContains: "id.tenancy.partition invalid",
		},
		"namespace mixed case": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Tenancy.Namespace = "Default"
				return artistId
			},
			errContains: "id.tenancy.namespace invalid",
		},
		"namespace name too long": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Tenancy.Namespace = strings.Repeat("n", resource.MaxNameLength+1)
				return artistId
			},
			errContains: "id.tenancy.namespace invalid",
		},
		"partition scoped resource with namespace": {
			modFn: func(_, recordLabelId, _ *pbresource.ID) *pbresource.ID {
				recordLabelId.Tenancy.Namespace = "ishouldnothaveanamespace"
				return recordLabelId
			},
			errContains: "cannot have a namespace",
		},
		"cluster scoped resource with partition": {
			modFn: func(_, _, executiveId *pbresource.ID) *pbresource.ID {
				executiveId.Tenancy.Partition = "ishouldnothaveapartition"
				executiveId.Tenancy.Namespace = ""
				return executiveId
			},
			errContains: "cannot have a partition",
		},
		"cluster scoped resource with namespace": {
			modFn: func(_, _, executiveId *pbresource.ID) *pbresource.ID {
				executiveId.Tenancy.Partition = ""
				executiveId.Tenancy.Namespace = "ishouldnothaveanamespace"
				return executiveId
			},
			errContains: "cannot have a namespace",
		},
	}

	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			run(t, client, tc)
		})
	}
}

func TestDelete_TypeNotRegistered(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().Run(t)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	// delete artist with unregistered type
	_, err = client.Delete(context.Background(), &pbresource.DeleteRequest{Id: artist.Id, Version: ""})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.ErrorContains(t, err, "not registered")
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
				require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String(), err)
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
			builder := svctest.NewResourceServiceBuilder().WithRegisterFns(demo.RegisterTypes)
			client := builder.Run(t)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			// Write test resource to delete.
			rsp, err := client.Write(context.Background(), &pbresource.WriteRequest{Resource: artist})
			require.NoError(t, err)

			// Mock is put in place after the above "write" since the "write" must also pass the ACL check.
			mockACLResolver := &svc.MockACLResolver{}
			mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
				Return(tc.authz, nil)
			builder.ServiceImpl().Config.ACLResolver = mockACLResolver

			// Exercise ACL.
			_, err = client.Delete(testContext(t), &pbresource.DeleteRequest{Id: rsp.Resource.Id})
			tc.assertErrFn(err)
		})
	}
}

func TestDelete_Success(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, client pbresource.ResourceServiceClient, tc deleteTestCase, modFn func(artistId, recordlabelId *pbresource.ID) *pbresource.ID) {
		ctx := context.Background()

		recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
		require.NoError(t, err)
		writeRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: recordLabel})
		require.NoError(t, err)
		recordLabel = writeRsp.Resource
		originalRecordLabelId := clone(recordLabel.Id)

		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)
		writeRsp, err = client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
		require.NoError(t, err)
		artist = writeRsp.Resource
		originalArtistId := clone(artist.Id)

		// Pick the resource to be deleted based on type's scope and mod tenancy
		// based on the tenancy test case.
		deleteId := modFn(artist.Id, recordLabel.Id)
		deleteReq := tc.deleteReqFn(recordLabel)
		if proto.Equal(deleteId.Type, demo.TypeV2Artist) {
			deleteReq = tc.deleteReqFn(artist)
		}

		// Delete
		_, err = client.Delete(ctx, deleteReq)
		require.NoError(t, err)

		// Verify deleted
		_, err = client.Read(ctx, &pbresource.ReadRequest{Id: deleteId})
		require.Error(t, err)
		require.Equal(t, codes.NotFound.String(), status.Code(err).String())

		// Derive tombstone name from resource that was deleted.
		tname := svc.TombstoneNameFor(originalRecordLabelId)
		if proto.Equal(deleteId.Type, demo.TypeV2Artist) {
			tname = svc.TombstoneNameFor(originalArtistId)
		}

		// Verify tombstone created
		_, err = client.Read(ctx, &pbresource.ReadRequest{
			Id: &pbresource.ID{
				Name:    tname,
				Type:    resource.TypeV1Tombstone,
				Tenancy: deleteReq.Id.Tenancy,
			},
		})
		require.NoError(t, err, "expected tombstone to be found")
	}

	for desc, tc := range deleteTestCases() {
		t.Run(desc, func(t *testing.T) {
			for tenancyDesc, modFn := range tenancyCases() {
				t.Run(tenancyDesc, func(t *testing.T) {
					client := svctest.NewResourceServiceBuilder().
						WithRegisterFns(demo.RegisterTypes).
						Run(t)
					run(t, client, tc, modFn)
				})
			}
		})
	}
}

func TestDelete_NonCAS_Retry(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Simulate conflicting versions by blocking the RPC after it has read the
	// current version of the resource, but before it tries to do a CAS delete
	// based on that version.
	backend := &blockOnceBackend{
		Backend: server.Backend,

		readCompletedCh: make(chan struct{}),
		blockCh:         make(chan struct{}),
	}
	server.Backend = backend

	deleteResultCh := make(chan error)
	go func() {
		_, err := client.Delete(testContext(t), &pbresource.DeleteRequest{Id: rsp1.Resource.Id, Version: ""})
		deleteResultCh <- err
	}()

	// Wait for the read, to ensure the Delete in the goroutine above has read the
	// current version of the resource.
	<-backend.readCompletedCh

	// Update the artist so that its version is different from the version read by Delete
	res = modifyArtist(t, rsp1.Resource)
	_, err = backend.WriteCAS(testContext(t), res)
	require.NoError(t, err)

	// Unblock the Delete by allowing the backend read to return and attempt a CAS delete.
	// The CAS delete should fail once, and they retry the backend read/delete cycle again
	// successfully.
	close(backend.blockCh)

	// Check that the delete succeeded anyway because of a retry.
	require.NoError(t, <-deleteResultCh)
}

func TestDelete_TombstoneDeletionDoesNotCreateNewTombstone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

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
			Name:    svc.TombstoneNameFor(artist.Id),
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

	run := func(t *testing.T, client pbresource.ResourceServiceClient, tc deleteTestCase) {
		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		// verify delete of non-existant or already deleted resource is a no-op
		_, err = client.Delete(context.Background(), tc.deleteReqFn(artist))
		require.NoError(t, err)
	}

	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	for desc, tc := range deleteTestCases() {
		t.Run(desc, func(t *testing.T) {
			run(t, client, tc)
		})
	}
}

func TestDelete_VersionMismatch(t *testing.T) {
	t.Parallel()

	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	rsp, err := client.Write(context.Background(), &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)

	// delete with a version that is different from the stored version
	_, err = client.Delete(context.Background(), &pbresource.DeleteRequest{Id: rsp.Resource.Id, Version: "non-existent-version"})
	require.Error(t, err)
	require.Equal(t, codes.Aborted.String(), status.Code(err).String())
	require.ErrorContains(t, err, "CAS operation failed")
}

func TestDelete_MarkedForDeletionWhenFinalizersPresent(t *testing.T) {
	ctx := context.Background()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	// Create a resource with a finalizer
	res := rtest.Resource(demo.TypeV1Artist, "manwithnoname").
		WithTenancy(resource.DefaultClusteredTenancy()).
		WithData(t, &pbdemo.Artist{Name: "Man With No Name"}).
		WithMeta(resource.FinalizerKey, "finalizer1").
		Write(t, client)

	// Delete it
	_, err := client.Delete(ctx, &pbresource.DeleteRequest{Id: res.Id})
	require.NoError(t, err)

	// Verify resource has been marked for deletion
	rsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: res.Id})
	require.NoError(t, err)
	require.True(t, resource.IsMarkedForDeletion(rsp.Resource))

	// Delete again - should be no-op
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: res.Id})
	require.NoError(t, err)

	// Verify no-op by checking version still the same
	rsp2, err := client.Read(ctx, &pbresource.ReadRequest{Id: res.Id})
	require.NoError(t, err)
	rtest.RequireVersionUnchanged(t, rsp2.Resource, rsp.Resource.Version)
}

func TestDelete_ImmediatelyDeletedAfterFinalizersRemoved(t *testing.T) {
	ctx := context.Background()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	// Create a resource with a finalizer
	res := rtest.Resource(demo.TypeV1Artist, "manwithnoname").
		WithTenancy(resource.DefaultClusteredTenancy()).
		WithData(t, &pbdemo.Artist{Name: "Man With No Name"}).
		WithMeta(resource.FinalizerKey, "finalizer1").
		Write(t, client)

	// Delete should mark it for deletion
	_, err := client.Delete(ctx, &pbresource.DeleteRequest{Id: res.Id})
	require.NoError(t, err)

	// Remove the finalizer
	rsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: res.Id})
	require.NoError(t, err)
	resource.RemoveFinalizer(rsp.Resource, "finalizer1")
	_, err = client.Write(ctx, &pbresource.WriteRequest{Resource: rsp.Resource})
	require.NoError(t, err)

	// Delete should be immediate
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: rsp.Resource.Id})
	require.NoError(t, err)

	// Verify deleted
	_, err = client.Read(ctx, &pbresource.ReadRequest{Id: rsp.Resource.Id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound.String(), status.Code(err).String())
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
