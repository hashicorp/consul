// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestRead_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)

	testCases := map[string]func(artistId, recordlabelId *pbresource.ID) *pbresource.ID{
		"no id": func(artistId, recordLabelId *pbresource.ID) *pbresource.ID { return nil },
		"no type": func(artistId, _ *pbresource.ID) *pbresource.ID {
			artistId.Type = nil
			return artistId
		},
		"no tenancy": func(artistId, _ *pbresource.ID) *pbresource.ID {
			artistId.Tenancy = nil
			return artistId
		},
		"no name": func(artistId, _ *pbresource.ID) *pbresource.ID {
			artistId.Name = ""
			return artistId
		},
		"partition scope with non-empty namespace": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
			recordLabelId.Tenancy.Namespace = "ishouldnothaveanamespace"
			return recordLabelId
		},
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
			require.NoError(t, err)

			// Each test case picks which resource to use based on the resource type's scope.
			req := &pbresource.ReadRequest{Id: modFn(artist.Id, recordLabel.Id)}

			_, err = client.Read(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		})
	}
}

func TestRead_TypeNotFound(t *testing.T) {
	server := NewServer(Config{Registry: resource.NewRegistry()})
	client := testClient(t, server)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Read(context.Background(), &pbresource.ReadRequest{Id: artist.Id})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.Artist not registered")
}

func TestRead_ResourceNotFound(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			tenancyCases := map[string]func(artistId, recordlabelId *pbresource.ID) *pbresource.ID{
				"resource not found by name": func(artistId, _ *pbresource.ID) *pbresource.ID {
					artistId.Name = "bogusname"
					return artistId
				},
				"partition not found when namespace scoped": func(artistId, _ *pbresource.ID) *pbresource.ID {
					id := clone(artistId)
					id.Tenancy.Partition = "boguspartition"
					return id
				},
				"namespace not found when namespace scoped": func(artistId, _ *pbresource.ID) *pbresource.ID {
					id := clone(artistId)
					id.Tenancy.Namespace = "bogusnamespace"
					return id
				},
				"partition not found when partition scoped": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
					id := clone(recordLabelId)
					id.Tenancy.Partition = "boguspartition"
					return id
				},
			}
			for tenancyDesc, modFn := range tenancyCases {
				t.Run(tenancyDesc, func(t *testing.T) {
					server := testServer(t)
					demo.RegisterTypes(server.Registry)
					client := testClient(t, server)

					recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
					require.NoError(t, err)
					recordLabel, err = server.Backend.WriteCAS(tc.ctx, recordLabel)
					require.NoError(t, err)

					artist, err := demo.GenerateV2Artist()
					require.NoError(t, err)
					artist, err = server.Backend.WriteCAS(tc.ctx, artist)
					require.NoError(t, err)

					// Each tenancy test case picks which resource to use based on the resource type's scope.
					_, err = client.Read(tc.ctx, &pbresource.ReadRequest{Id: modFn(artist.Id, recordLabel.Id)})
					require.Error(t, err)
					require.Equal(t, codes.NotFound.String(), status.Code(err).String())
					require.Contains(t, err.Error(), "resource not found")
				})
			}
		})
	}
}

func TestRead_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)

			demo.RegisterTypes(server.Registry)
			client := testClient(t, server)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			_, err = server.Backend.WriteCAS(tc.ctx, artist)
			require.NoError(t, err)

			id := clone(artist.Id)
			id.Type = demo.TypeV1Artist

			_, err = client.Read(tc.ctx, &pbresource.ReadRequest{Id: id})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource was requested with GroupVersion")
		})
	}
}

func TestRead_Success(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			tenancyCases := map[string]func(artistId, recordlabelId *pbresource.ID) *pbresource.ID{
				"namespaced resource provides nonempty partition and namespace": func(artistId, recordLabelId *pbresource.ID) *pbresource.ID {
					return artistId
				},
				"namespaced resource provides uppercase namespace and partition": func(artistId, _ *pbresource.ID) *pbresource.ID {
					id := clone(artistId)
					id.Tenancy.Partition = strings.ToUpper(artistId.Tenancy.Partition)
					id.Tenancy.Namespace = strings.ToUpper(artistId.Tenancy.Namespace)
					return id
				},
				"namespaced resource inherits tokens namespace when empty": func(artistId, _ *pbresource.ID) *pbresource.ID {
					id := clone(artistId)
					id.Tenancy.Namespace = ""
					return id
				},
				"namespaced resource inherits tokens partition when empty": func(artistId, _ *pbresource.ID) *pbresource.ID {
					id := clone(artistId)
					id.Tenancy.Partition = ""
					return id
				},
				"namespaced resource inherits tokens partition and namespace when empty": func(artistId, _ *pbresource.ID) *pbresource.ID {
					id := clone(artistId)
					id.Tenancy.Partition = ""
					id.Tenancy.Namespace = ""
					return id
				},
				"partitioned resource provides nonempty partition": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
					return recordLabelId
				},
				"partitioned resource provides uppercase partition": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
					id := clone(recordLabelId)
					id.Tenancy.Partition = strings.ToUpper(recordLabelId.Tenancy.Partition)
					return id
				},
				"partitioned resource inherits tokens partition when empty": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
					id := clone(recordLabelId)
					id.Tenancy.Partition = ""
					return id
				},
			}
			for tenancyDesc, modFn := range tenancyCases {
				t.Run(tenancyDesc, func(t *testing.T) {
					server := testServer(t)
					demo.RegisterTypes(server.Registry)
					client := testClient(t, server)

					recordLabel, err := demo.GenerateV1RecordLabel("LoonyTunes")
					require.NoError(t, err)
					recordLabel, err = server.Backend.WriteCAS(tc.ctx, recordLabel)
					require.NoError(t, err)

					artist, err := demo.GenerateV2Artist()
					require.NoError(t, err)
					artist, err = server.Backend.WriteCAS(tc.ctx, artist)
					require.NoError(t, err)

					// Each tenancy test case picks which resource to use based on the resource type's scope.
					req := &pbresource.ReadRequest{Id: modFn(artist.Id, recordLabel.Id)}
					rsp, err := client.Read(tc.ctx, req)
					require.NoError(t, err)

					switch {
					case proto.Equal(rsp.Resource.Id.Type, demo.TypeV2Artist):
						prototest.AssertDeepEqual(t, artist, rsp.Resource)
					case proto.Equal(rsp.Resource.Id.Type, demo.TypeV1RecordLabel):
						prototest.AssertDeepEqual(t, recordLabel, rsp.Resource)
					default:
						require.Fail(t, "unexpected resource type")
					}
				})
			}
		})
	}
}

func TestRead_VerifyReadConsistencyArg(t *testing.T) {
	// Uses a mockBackend instead of the inmem Backend to verify the ReadConsistency argument is set correctly.
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			mockBackend := NewMockBackend(t)
			server.Backend = mockBackend
			demo.RegisterTypes(server.Registry)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			mockBackend.On("Read", mock.Anything, mock.Anything, mock.Anything).Return(artist, nil)
			client := testClient(t, server)

			rsp, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: artist.Id})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, artist, rsp.Resource)
			mockBackend.AssertCalled(t, "Read", mock.Anything, tc.consistency, mock.Anything)
		})
	}
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestRead_ACLs(t *testing.T) {
	type testCase struct {
		authz resolver.Result
		code  codes.Code
	}
	testcases := map[string]testCase{
		"read hook denied": {
			authz: AuthorizerFrom(t, demo.ArtistV1ReadPolicy),
			code:  codes.PermissionDenied,
		},
		"read hook allowed": {
			authz: AuthorizerFrom(t, demo.ArtistV2ReadPolicy),
			code:  codes.NotFound,
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
			_, err = client.Read(testContext(t), &pbresource.ReadRequest{Id: artist.Id})
			require.Error(t, err)
			require.Equal(t, tc.code.String(), status.Code(err).String())
		})
	}
}

type readTestCase struct {
	consistency storage.ReadConsistency
	ctx         context.Context
}

func readTestCases() map[string]readTestCase {
	return map[string]readTestCase{
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
