// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov1 "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestWrite_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	testCases := map[string]func(artist, recordLabel *pbresource.Resource) *pbresource.Resource{
		"no resource": func(artist, recordLabel *pbresource.Resource) *pbresource.Resource { return nil },
		"no id": func(artist, _ *pbresource.Resource) *pbresource.Resource {
			artist.Id = nil
			return artist
		},
		"no type": func(artist, _ *pbresource.Resource) *pbresource.Resource {
			artist.Id.Type = nil
			return artist
		},
		"no tenancy": func(artist, _ *pbresource.Resource) *pbresource.Resource {
			artist.Id.Tenancy = nil
			return artist
		},
		"no name": func(artist, _ *pbresource.Resource) *pbresource.Resource {
			artist.Id.Name = ""
			return artist
		},
		"no data": func(artist, _ *pbresource.Resource) *pbresource.Resource {
			artist.Data = nil
			return artist
		},
		"wrong data type": func(artist, _ *pbresource.Resource) *pbresource.Resource {
			var err error
			artist.Data, err = anypb.New(&pbdemov2.Album{})
			require.NoError(t, err)
			return artist
		},
		"fail validation hook": func(artist, _ *pbresource.Resource) *pbresource.Resource {
			buffer := &pbdemov2.Artist{}
			require.NoError(t, artist.Data.UnmarshalTo(buffer))
			buffer.Name = "" // name cannot be empty
			require.NoError(t, artist.Data.MarshalFrom(buffer))
			return artist
		},
		"partition scope with non-empty namespace": func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
			recordLabel.Id.Tenancy.Namespace = "bogus"
			return recordLabel
		},
		// TODO(spatel): add cluster scope tests when we have an actual cluster scoped resource (e.g. partition)
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
			require.NoError(t, err)

			req := &pbresource.WriteRequest{Resource: modFn(artist, recordLabel)}
			_, err = client.Write(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		})
	}
}

func TestWrite_OwnerValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	type testCase struct {
		modReqFn      func(req *pbresource.WriteRequest)
		errorContains string
	}
	testCases := map[string]testCase{
		"no owner type": {
			modReqFn:      func(req *pbresource.WriteRequest) { req.Resource.Owner.Type = nil },
			errorContains: "resource.owner.type",
		},
		"no owner tenancy": {
			modReqFn:      func(req *pbresource.WriteRequest) { req.Resource.Owner.Tenancy = nil },
			errorContains: "resource.owner.tenancy",
		},
		"no owner name": {
			modReqFn:      func(req *pbresource.WriteRequest) { req.Resource.Owner.Name = "" },
			errorContains: "resource.owner.name",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			album, err := demo.GenerateV2Album(artist.Id)
			require.NoError(t, err)

			albumReq := &pbresource.WriteRequest{Resource: album}
			tc.modReqFn(albumReq)

			_, err = client.Write(testContext(t), albumReq)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.ErrorContains(t, err, tc.errorContains)
		})
	}
}

func TestWrite_TypeNotFound(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

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
			server := testServer(t)
			client := testClient(t, server)

			mockACLResolver := &MockACLResolver{}
			mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
				Return(tc.authz, nil)
			server.ACLResolver = mockACLResolver
			demo.RegisterTypes(server.Registry)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			// exercise ACL
			_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
			tc.assertErrFn(err)
		})
	}
}

func TestWrite_Mutate(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)

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
	testCases := map[string]struct {
		modFn           func(artist, recordLabel *pbresource.Resource) *pbresource.Resource
		expectedTenancy *pbresource.Tenancy
	}{
		"namespaced resource provides nonempty partition and namespace": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"namespaced resource provides uppercase partition and namespace": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = strings.ToUpper(artist.Id.Tenancy.Partition)
				artist.Id.Tenancy.Namespace = strings.ToUpper(artist.Id.Tenancy.Namespace)
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"namespaced resource inherits tokens partition when empty": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = ""
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"namespaced resource inherits tokens namespace when empty": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Namespace = ""
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"namespaced resource inherits tokens partition and namespace when empty": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = ""
				artist.Id.Tenancy.Namespace = ""
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"partitioned resource provides nonempty partition": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				return recordLabel
			},
			expectedTenancy: resource.DefaultPartitionedTenancy(),
		},
		"partitioned resource provides uppercase partition": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				recordLabel.Id.Tenancy.Partition = strings.ToUpper(recordLabel.Id.Tenancy.Partition)
				return recordLabel
			},
			expectedTenancy: resource.DefaultPartitionedTenancy(),
		},
		"partitioned resource inherits tokens partition when empty": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				recordLabel.Id.Tenancy.Partition = ""
				return recordLabel
			},
			expectedTenancy: resource.DefaultPartitionedTenancy(),
		},
		// TODO(spatel): Add cluster scope tests when we have an actual cluster scoped resource (e.g. partition)
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			client := testClient(t, server)
			demo.RegisterTypes(server.Registry)

			recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
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

func TestWrite_Create_Invalid_Tenancy(t *testing.T) {
	testCases := map[string]struct {
		modFn       func(artist, recordLabel *pbresource.Resource) *pbresource.Resource
		errCode     codes.Code
		errContains string
	}{
		"namespaced resource provides nonexistant partition": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = "boguspartition"
				return artist
			},
			errCode:     codes.InvalidArgument,
			errContains: "partition",
		},
		"namespaced resource provides nonexistant namespace": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Namespace = "bogusnamespace"
				return artist
			},
			errCode:     codes.InvalidArgument,
			errContains: "namespace",
		},
		"partitioned resource provides nonexistant partition": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				recordLabel.Id.Tenancy.Partition = "boguspartition"
				return recordLabel
			},
			errCode:     codes.InvalidArgument,
			errContains: "partition",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			client := testClient(t, server)
			demo.RegisterTypes(server.Registry)

			recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
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

func TestWrite_Tenancy_MarkedForDeletion(t *testing.T) {
	// Verify resource write fails when its partition or namespace is marked for deletion.
	testCases := map[string]struct {
		modFn       func(artist, recordLabel *pbresource.Resource, mockTenancyBridge *MockTenancyBridge) *pbresource.Resource
		errContains string
	}{
		"namespaced resources partition marked for deletion": {
			modFn: func(artist, _ *pbresource.Resource, mockTenancyBridge *MockTenancyBridge) *pbresource.Resource {
				mockTenancyBridge.On("IsPartitionMarkedForDeletion", "part1").Return(true, nil)
				return artist
			},
			errContains: "partition marked for deletion",
		},
		"namespaced resources namespace marked for deletion": {
			modFn: func(artist, _ *pbresource.Resource, mockTenancyBridge *MockTenancyBridge) *pbresource.Resource {
				mockTenancyBridge.On("IsPartitionMarkedForDeletion", "part1").Return(false, nil)
				mockTenancyBridge.On("IsNamespaceMarkedForDeletion", "part1", "ns1").Return(true, nil)
				return artist
			},
			errContains: "namespace marked for deletion",
		},
		"partitioned resources partition marked for deletion": {
			modFn: func(_, recordLabel *pbresource.Resource, mockTenancyBridge *MockTenancyBridge) *pbresource.Resource {
				mockTenancyBridge.On("IsPartitionMarkedForDeletion", "part1").Return(true, nil)
				return recordLabel
			},
			errContains: "partition marked for deletion",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			client := testClient(t, server)
			demo.RegisterTypes(server.Registry)
			recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
			require.NoError(t, err)
			recordLabel.Id.Tenancy.Partition = "part1"

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)
			artist.Id.Tenancy.Partition = "part1"
			artist.Id.Tenancy.Namespace = "ns1"

			mockTenancyBridge := &MockTenancyBridge{}
			mockTenancyBridge.On("PartitionExists", "part1").Return(true, nil)
			mockTenancyBridge.On("NamespaceExists", "part1", "ns1").Return(true, nil)
			server.V1TenancyBridge = mockTenancyBridge

			_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: tc.modFn(artist, recordLabel, mockTenancyBridge)})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestWrite_CASUpdate_Success(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	statusRsp, err := client.WriteStatus(testContext(t), validWriteStatusRequest(t, rsp1.Resource))
	require.NoError(t, err)
	res = statusRsp.Resource

	// Passing the staus unmodified should be fine.
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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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

		readCh:  make(chan struct{}),
		blockCh: make(chan struct{}),
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
	<-backend.readCh

	// Update the resource.
	res = modifyArtist(t, rsp1.Resource)
	_, err = backend.WriteCAS(testContext(t), res)
	require.NoError(t, err)

	// Unblock the read.
	close(backend.blockCh)

	// Check that the write succeeded anyway because of a retry.
	require.NoError(t, <-errCh)
}

func TestWrite_Owner_Immutable(t *testing.T) {
	// Use of proto.Equal(..) in implementation covers all permutations
	// (nil -> non-nil, non-nil -> nil, owner1 -> owner2) so only the first one
	// is tested.
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

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

type blockOnceBackend struct {
	storage.Backend

	done    uint32
	readCh  chan struct{}
	blockCh chan struct{}
}

func (b *blockOnceBackend) Read(ctx context.Context, consistency storage.ReadConsistency, id *pbresource.ID) (*pbresource.Resource, error) {
	res, err := b.Backend.Read(ctx, consistency, id)

	// Block for exactly one call to Read. All subsequent calls (including those
	// concurrent to the blocked call) will return immediately.
	if atomic.CompareAndSwapUint32(&b.done, 0, 1) {
		close(b.readCh)
		<-b.blockCh
	}

	return res, err
}
