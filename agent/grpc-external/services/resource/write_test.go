// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl/resolver"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	pbdemov1 "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/proto/private/prototest"
)

// TODO: Update all tests to use true/false table test for v2tenancy

func TestWrite_InputValidation(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	for desc, tc := range resourceValidTestCases(t) {
		t.Run(desc, func(t *testing.T) {
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
			require.NoError(t, err)

			req := &pbresource.WriteRequest{Resource: tc.modFn(artist, recordLabel)}
			_, err = client.Write(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestWrite_OwnerValidation(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	testCases := ownerValidationTestCases(t)

	// This is not part of ownerValidationTestCases because it is a special case
	// that only gets caught deeper into the write path.
	testCases["no owner tenancy"] = ownerValidTestCase{
		modFn:         func(res *pbresource.Resource) { res.Owner.Tenancy = nil },
		errorContains: "resource.owner does not exist",
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			album, err := demo.GenerateV2Album(artist.Id)
			require.NoError(t, err)

			tc.modFn(album)

			_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.ErrorContains(t, err, tc.errorContains)
		})
	}
}

func TestWrite_TypeNotFound(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.Artist not registered")
}

func TestWrite_ACLs(t *testing.T) {
	type testCase struct {
		authz       resolver.Result
		assertErrFn func(error)
	}
	testcases := map[string]testCase{
		"write denied": {
			authz: AuthorizerFrom(t, demo.ArtistV1WritePolicy),
			assertErrFn: func(err error) {
				require.Error(t, err)
				require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
			},
		},
		"write allowed": {
			authz: AuthorizerFrom(t, demo.ArtistV2WritePolicy),
			assertErrFn: func(err error) {
				require.NoError(t, err)
			},
		},
	}

	for desc, tc := range testcases {
		t.Run(desc, func(t *testing.T) {
			mockACLResolver := &svc.MockACLResolver{}
			mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
				Return(tc.authz, nil)

			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				WithACLResolver(mockACLResolver).
				Run(t)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			// exercise ACL
			_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
			tc.assertErrFn(err)
		})
	}
}

func TestWrite_Mutate(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	artistData := &pbdemov2.Artist{}
	artist.Data.UnmarshalTo(artistData)
	require.NoError(t, err)

	// mutate hook sets genre to disco when unspecified
	artistData.Genre = pbdemov2.Genre_GENRE_UNSPECIFIED
	artist.Data.MarshalFrom(artistData)
	require.NoError(t, err)

	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)

	// verify mutate hook set genre to disco
	require.NoError(t, rsp.Resource.Data.UnmarshalTo(artistData))
	require.Equal(t, pbdemov2.Genre_GENRE_DISCO, artistData.Genre)
}

func TestWrite_Create_Success(t *testing.T) {
	for desc, tc := range mavOrWriteSuccessTestCases(t) {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
			require.NoError(t, err)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: tc.modFn(artist, recordLabel)})
			require.NoError(t, err)
			require.NotEmpty(t, rsp.Resource.Version, "resource should have version")
			require.NotEmpty(t, rsp.Resource.Id.Uid, "resource id should have uid")
			require.NotEmpty(t, rsp.Resource.Generation, "resource should have generation")
			prototest.AssertDeepEqual(t, tc.expectedTenancy, rsp.Resource.Id.Tenancy)
		})
	}
}

func TestWrite_Create_Tenancy_NotFound(t *testing.T) {
	for desc, tc := range mavOrWriteTenancyNotFoundTestCases(t) {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithV2Tenancy(true).
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
			require.NoError(t, err)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: tc.modFn(artist, recordLabel)})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestWrite_Create_With_DeletionTimestamp_Fails(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithV2Tenancy(true).
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res := rtest.Resource(demo.TypeV1Artist, "blur").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbdemov1.Artist{Name: "Blur"}).
		WithMeta(resource.DeletionTimestampKey, time.Now().Format(time.RFC3339)).
		Build()

	_, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), resource.DeletionTimestampKey)
}

func TestWrite_Create_With_TenancyMarkedForDeletion_Fails(t *testing.T) {
	for desc, tc := range mavOrWriteTenancyMarkedForDeletionTestCases(t) {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			client := testClient(t, server)
			demo.RegisterTypes(server.Registry)

			recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
			require.NoError(t, err)
			recordLabel.Id.Tenancy.Partition = "ap1"

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)
			artist.Id.Tenancy.Partition = "ap1"
			artist.Id.Tenancy.Namespace = "ns1"

			mockTenancyBridge := &svc.MockTenancyBridge{}
			mockTenancyBridge.On("PartitionExists", "ap1").Return(true, nil)
			mockTenancyBridge.On("NamespaceExists", "ap1", "ns1").Return(true, nil)
			server.TenancyBridge = mockTenancyBridge

			_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: tc.modFn(artist, recordLabel, mockTenancyBridge)})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestWrite_CASUpdate_Success(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{
		Resource: modifyArtist(t, rsp1.Resource),
	})
	require.NoError(t, err)

	require.Equal(t, rsp1.Resource.Id.Uid, rsp2.Resource.Id.Uid)
	require.NotEqual(t, rsp1.Resource.Version, rsp2.Resource.Version)
	require.NotEqual(t, rsp1.Resource.Generation, rsp2.Resource.Generation)
}

func TestWrite_ResourceCreation_StatusProvided(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	res.Status = map[string]*pbresource.Status{
		"consul.io/some-controller": {ObservedGeneration: ulid.Make().String()},
	}

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "WriteStatus endpoint")
}

func TestWrite_CASUpdate_Failure(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = modifyArtist(t, rsp1.Resource)
	res.Version = "wrong-version"

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.Aborted.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "CAS operation failed")
}

func TestWrite_Update_WrongUid(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = modifyArtist(t, rsp1.Resource)
	res.Id.Uid = "wrong-uid"

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "uid doesn't match")
}

func TestWrite_Update_StatusModified(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	statusRsp, err := client.WriteStatus(testContext(t), validWriteStatusRequest(t, rsp1.Resource))
	require.NoError(t, err)
	res = statusRsp.Resource

	// Passing the status unmodified should be fine.
	rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Attempting to modify the status should return an error.
	res = rsp2.Resource
	res.Status["consul.io/other-controller"] = &pbresource.Status{ObservedGeneration: res.Generation}

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "WriteStatus endpoint")
}

func TestWrite_Update_NilStatus(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	statusRsp, err := client.WriteStatus(testContext(t), validWriteStatusRequest(t, rsp1.Resource))
	require.NoError(t, err)

	// Passing a nil status should be fine (and carry over the old status).
	res = statusRsp.Resource
	res.Status = nil

	rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	require.NotEmpty(t, rsp2.Resource.Status)
}

func TestWrite_Update_NoUid(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = modifyArtist(t, rsp1.Resource)
	res.Id.Uid = ""

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
}

func TestWrite_Update_GroupVersion(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = rsp1.Resource
	res.Id.Type = demo.TypeV1Artist

	// translate artistV2 to artistV1
	var artistV2 pbdemov2.Artist
	require.NoError(t, res.Data.UnmarshalTo(&artistV2))
	artistV1 := &pbdemov1.Artist{
		Name:         artistV2.Name,
		Description:  "some awesome band",
		Genre:        pbdemov1.Genre_GENRE_JAZZ,
		GroupMembers: int32(len(artistV2.GroupMembers)),
	}
	res.Data.MarshalFrom(artistV1)

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
}

func TestWrite_NonCASUpdate_Success(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = modifyArtist(t, rsp1.Resource)
	res.Version = ""

	rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	require.NotEmpty(t, rsp2.Resource.Version)
	require.NotEqual(t, rsp1.Resource.Version, rsp2.Resource.Version)
}

func TestWrite_NonCASUpdate_Retry(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Simulate conflicting writes by blocking the RPC after it has read the
	// current version of the resource, but before it tries to make a write.
	backend := &blockOnceBackend{
		Backend: server.Backend,

		readCompletedCh: make(chan struct{}),
		blockCh:         make(chan struct{}),
	}
	server.Backend = backend

	errCh := make(chan error)
	go func() {
		res := modifyArtist(t, rsp1.Resource)
		res.Version = ""

		_, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		errCh <- err
	}()

	// Wait for the read, to ensure the Write in the goroutine above has read the
	// current version of the resource.
	<-backend.readCompletedCh

	// Update the resource.
	res = modifyArtist(t, rsp1.Resource)
	_, err = backend.WriteCAS(testContext(t), res)
	require.NoError(t, err)

	// Unblock the read.
	close(backend.blockCh)

	// Check that the write succeeded anyway because of a retry.
	require.NoError(t, <-errCh)
}

func TestWrite_NoData(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	res, err := demo.GenerateV1Concept("jazz")
	require.NoError(t, err)

	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	require.NotEmpty(t, rsp.Resource.Version)
	require.Equal(t, rsp.Resource.Id.Name, "jazz")
}

func TestWrite_Owner_Immutable(t *testing.T) {
	// Use of proto.Equal(..) in implementation covers all permutations
	// (nil -> non-nil, non-nil -> nil, owner1 -> owner2) so only the first one
	// is tested.
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)
	artist = rsp1.Resource

	// create album with no owner
	album, err := demo.GenerateV2Album(rsp1.Resource.Id)
	require.NoError(t, err)
	album.Owner = nil
	rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
	require.NoError(t, err)

	// setting owner on update should fail
	album = rsp2.Resource
	album.Owner = artist.Id
	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.ErrorContains(t, err, "owner cannot be changed")
}

func TestWrite_Owner_Uid(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	t.Run("uid given", func(t *testing.T) {
		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		album, err := demo.GenerateV2Album(artist.Id)
		require.NoError(t, err)
		album.Owner.Uid = ulid.Make().String()

		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
		require.NoError(t, err)
	})

	t.Run("no uid - owner not found", func(t *testing.T) {
		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		album, err := demo.GenerateV2Album(artist.Id)
		require.NoError(t, err)

		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	})

	t.Run("no uid - automatically resolved", func(t *testing.T) {
		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
		require.NoError(t, err)
		artist = rsp1.Resource

		album, err := demo.GenerateV2Album(clone(artist.Id))
		require.NoError(t, err)

		// Blank out the owner Uid to check it gets automatically filled in.
		album.Owner.Uid = ""

		rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
		require.NoError(t, err)
		require.NotEmpty(t, rsp2.Resource.Owner.Uid)
		require.Equal(t, artist.Id.Uid, rsp2.Resource.Owner.Uid)
	})

	t.Run("no-uid - update auto resolve", func(t *testing.T) {
		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		uid := ulid.Make().String()
		album, err := demo.GenerateV2Album(artist.Id)
		require.NoError(t, err)
		album.Owner.Uid = uid

		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
		require.NoError(t, err)

		// unset the uid and rewrite the resource
		album.Owner.Uid = ""
		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
		require.NoError(t, err)
		require.Equal(t, uid, rsp.GetResource().GetOwner().GetUid())
	})
}

func TestEnsureFinalizerRemoved(t *testing.T) {
	type testCase struct {
		mod         func(input, existing *pbresource.Resource)
		errContains string
	}

	testCases := map[string]testCase{
		"one finalizer removed from input": {
			mod: func(input, existing *pbresource.Resource) {
				resource.AddFinalizer(existing, "f1")
				resource.AddFinalizer(existing, "f2")
				resource.AddFinalizer(input, "f1")
			},
		},
		"all finalizers removed from input": {
			mod: func(input, existing *pbresource.Resource) {
				resource.AddFinalizer(existing, "f1")
				resource.AddFinalizer(existing, "f2")
				resource.AddFinalizer(input, "f1")
				resource.RemoveFinalizer(input, "f1")
			},
		},
		"all finalizers removed from input and no finalizer key": {
			mod: func(input, existing *pbresource.Resource) {
				resource.AddFinalizer(existing, "f1")
				resource.AddFinalizer(existing, "f2")
			},
		},
		"no finalizers removed from input": {
			mod: func(input, existing *pbresource.Resource) {
				resource.AddFinalizer(existing, "f1")
				resource.AddFinalizer(input, "f1")
			},
			errContains: "expected at least one finalizer to be removed",
		},
		"input finalizers not proper subset of existing": {
			mod: func(input, existing *pbresource.Resource) {
				resource.AddFinalizer(existing, "f1")
				resource.AddFinalizer(existing, "f2")
				resource.AddFinalizer(input, "f3")
			},
			errContains: "expected at least one finalizer to be removed",
		},
		"existing has no finalizers for input to remove": {
			mod: func(input, existing *pbresource.Resource) {
				resource.AddFinalizer(input, "f3")
			},
			errContains: "expected at least one finalizer to be removed",
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			input := rtest.Resource(demo.TypeV1Artist, "artist1").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, &pbdemov1.Artist{Name: "artist1"}).
				WithMeta(resource.DeletionTimestampKey, "someTimestamp").
				Build()

			existing := rtest.Resource(demo.TypeV1Artist, "artist1").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, &pbdemov1.Artist{Name: "artist1"}).
				WithMeta(resource.DeletionTimestampKey, "someTimestamp").
				Build()

			tc.mod(input, existing)

			err := svc.EnsureFinalizerRemoved(input, existing)
			if tc.errContains != "" {
				require.Error(t, err)
				require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
				require.ErrorContains(t, err, tc.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWrite_ResourceFrozenAfterMarkedForDeletion(t *testing.T) {
	type testCase struct {
		modFn       func(res *pbresource.Resource)
		errContains string
	}
	testCases := map[string]testCase{
		"no-op write rejected": {
			modFn:       func(res *pbresource.Resource) {},
			errContains: "cannot no-op write resource marked for deletion",
		},
		"remove one finalizer": {
			modFn: func(res *pbresource.Resource) {
				resource.RemoveFinalizer(res, "finalizer1")
			},
		},
		"remove all finalizers": {
			modFn: func(res *pbresource.Resource) {
				resource.RemoveFinalizer(res, "finalizer1")
				resource.RemoveFinalizer(res, "finalizer2")
			},
		},
		"adding finalizer fails": {
			modFn: func(res *pbresource.Resource) {
				resource.AddFinalizer(res, "finalizer3")
			},
			errContains: "expected at least one finalizer to be removed",
		},
		"remove deletionTimestamp fails": {
			modFn: func(res *pbresource.Resource) {
				delete(res.Metadata, resource.DeletionTimestampKey)
			},
			errContains: "cannot remove deletionTimestamp",
		},
		"modify deletionTimestamp fails": {
			modFn: func(res *pbresource.Resource) {
				res.Metadata[resource.DeletionTimestampKey] = "bad"
			},
			errContains: "cannot modify deletionTimestamp",
		},
		"modify data fails": {
			modFn: func(res *pbresource.Resource) {
				var err error
				res.Data, err = anypb.New(&pbdemo.Artist{Name: "New Order"})
				require.NoError(t, err)
			},
			errContains: "cannot modify data",
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithV2Tenancy(true).
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			// Create a resource with finalizers
			res := rtest.Resource(demo.TypeV1Artist, "joydivision").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, &pbdemo.Artist{Name: "Joy Division"}).
				WithMeta(resource.FinalizerKey, "finalizer1 finalizer2").
				Write(t, client)

			// Mark for deletion - resource should now be frozen
			_, err := client.Delete(context.Background(), &pbresource.DeleteRequest{Id: res.Id})
			require.NoError(t, err)

			// Verify marked for deletion
			rsp, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: res.Id})
			require.NoError(t, err)
			require.True(t, resource.IsMarkedForDeletion(rsp.Resource))

			// Apply test case mods
			tc.modFn(rsp.Resource)

			// Verify write results
			_, err = client.Write(context.Background(), &pbresource.WriteRequest{Resource: rsp.Resource})
			if tc.errContains == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
				require.ErrorContains(t, err, tc.errContains)
			}
		})
	}
}

func TestWrite_NonCASWritePreservesFinalizers(t *testing.T) {
	type testCase struct {
		existingMeta map[string]string
		inputMeta    map[string]string
		expectedMeta map[string]string
	}
	testCases := map[string]testCase{
		"input nil metadata preserves existing finalizers": {
			inputMeta:    nil,
			existingMeta: map[string]string{resource.FinalizerKey: "finalizer1 finalizer2"},
			expectedMeta: map[string]string{resource.FinalizerKey: "finalizer1 finalizer2"},
		},
		"input metadata and no finalizer key preserves existing finalizers": {
			inputMeta:    map[string]string{},
			existingMeta: map[string]string{resource.FinalizerKey: "finalizer1 finalizer2"},
			expectedMeta: map[string]string{resource.FinalizerKey: "finalizer1 finalizer2"},
		},
		"input metadata and with empty finalizer key overwrites existing finalizers": {
			inputMeta:    map[string]string{resource.FinalizerKey: ""},
			existingMeta: map[string]string{resource.FinalizerKey: "finalizer1 finalizer2"},
			expectedMeta: map[string]string{resource.FinalizerKey: ""},
		},
		"input metadata with one finalizer key overwrites multiple existing finalizers": {
			inputMeta:    map[string]string{resource.FinalizerKey: "finalizer2"},
			existingMeta: map[string]string{resource.FinalizerKey: "finalizer1 finalizer2"},
			expectedMeta: map[string]string{resource.FinalizerKey: "finalizer2"},
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithV2Tenancy(true).
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			// Create the resource based on tc.existingMetadata
			builder := rtest.Resource(demo.TypeV1Artist, "joydivision").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, &pbdemo.Artist{Name: "Joy"})

			if tc.existingMeta != nil {
				for k, v := range tc.existingMeta {
					builder.WithMeta(k, v)
				}
			}
			res := builder.Write(t, client)

			// Build resource for user write based on tc.inputMetadata
			builder = rtest.Resource(demo.TypeV1Artist, res.Id.Name).
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, &pbdemo.Artist{Name: "Joy Division"})

			if tc.inputMeta != nil {
				for k, v := range tc.inputMeta {
					builder.WithMeta(k, v)
				}
			}
			userRes := builder.Build()

			// Perform the user write
			rsp, err := client.Write(context.Background(), &pbresource.WriteRequest{Resource: userRes})
			require.NoError(t, err)

			// Verify write result preserved metadata based on testcase.expecteMetadata
			for k := range tc.expectedMeta {
				require.Equal(t, tc.expectedMeta[k], rsp.Resource.Metadata[k])
			}
			require.Equal(t, len(tc.expectedMeta), len(rsp.Resource.Metadata))
		})
	}
}

func TestWrite_NonCASWritePreservesDeletionTimestamp(t *testing.T) {
	type testCase struct {
		existingMeta map[string]string
		inputMeta    map[string]string
		expectedMeta map[string]string
	}

	// deletionTimestamp has to be generated via Delete() call and can't be embedded in testdata
	// even though testcase desc refers to it.
	testCases := map[string]testCase{
		"input metadata no deletion timestamp preserves existing deletion timestamp and removes single finalizer": {
			inputMeta:    map[string]string{resource.FinalizerKey: "finalizer1"},
			existingMeta: map[string]string{resource.FinalizerKey: "finalizer1 finalizer2"},
			expectedMeta: map[string]string{resource.FinalizerKey: "finalizer1"},
		},
		"input metadata no deletion timestamp preserves existing deletion timestamp and removes all finalizers": {
			inputMeta:    map[string]string{resource.FinalizerKey: ""},
			existingMeta: map[string]string{resource.FinalizerKey: "finalizer1 finalizer2"},
			expectedMeta: map[string]string{resource.FinalizerKey: ""},
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithV2Tenancy(true).
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			// Create the resource based on tc.existingMetadata
			builder := rtest.Resource(demo.TypeV1Artist, "joydivision").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, &pbdemo.Artist{Name: "Joy Division"})

			if tc.existingMeta != nil {
				for k, v := range tc.existingMeta {
					builder.WithMeta(k, v)
				}
			}
			res := builder.Write(t, client)

			// Mark for deletion
			_, err := client.Delete(context.Background(), &pbresource.DeleteRequest{Id: res.Id})
			require.NoError(t, err)

			// Re-read the deleted res for future comparison of deletionTimestamp
			delRsp, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: res.Id})
			require.NoError(t, err)

			// Build resource for user write based on tc.inputMetadata
			builder = rtest.Resource(demo.TypeV1Artist, res.Id.Name).
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, &pbdemo.Artist{Name: "Joy Division"})

			if tc.inputMeta != nil {
				for k, v := range tc.inputMeta {
					builder.WithMeta(k, v)
				}
			}
			userRes := builder.Build()

			// Perform the non-CAS user write
			rsp, err := client.Write(context.Background(), &pbresource.WriteRequest{Resource: userRes})
			require.NoError(t, err)

			// Verify write result preserved metadata based on testcase.expectedMetadata
			for k := range tc.expectedMeta {
				require.Equal(t, tc.expectedMeta[k], rsp.Resource.Metadata[k])
			}
			// Verify deletion timestamp preserved even though it wasn't passed in to the write
			require.Equal(t, delRsp.Resource.Metadata[resource.DeletionTimestampKey], rsp.Resource.Metadata[resource.DeletionTimestampKey])
		})
	}
}
