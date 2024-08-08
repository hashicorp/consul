// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestRead_InputValidation(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	type testCase struct {
		modFn       func(artistId, recordlabelId, executiveId *pbresource.ID) *pbresource.ID
		errContains string
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
		"name is mixed case": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Name = "MixedCaseNotAllowed"
				return artistId
			},
			errContains: "id.name invalid",
		},
		"name too long": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Name = strings.Repeat("a", resource.MaxNameLength+1)
				return artistId
			},
			errContains: "id.name invalid",
		},
		"partition is mixed case": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Tenancy.Partition = "Default"
				return artistId
			},
			errContains: "id.tenancy.partition invalid",
		},
		"partition too long": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Tenancy.Partition = strings.Repeat("p", resource.MaxNameLength+1)
				return artistId
			},
			errContains: "id.tenancy.partition invalid",
		},
		"namespace is mixed case": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Tenancy.Namespace = "Default"
				return artistId
			},
			errContains: "id.tenancy.namespace invalid",
		},
		"namespace too long": {
			modFn: func(artistId, _, _ *pbresource.ID) *pbresource.ID {
				artistId.Tenancy.Namespace = strings.Repeat("n", resource.MaxNameLength+1)
				return artistId
			},
			errContains: "id.tenancy.namespace invalid",
		},
		"partition scope with non-empty namespace": {
			modFn: func(_, recordLabelId, _ *pbresource.ID) *pbresource.ID {
				recordLabelId.Tenancy.Namespace = "ishouldnothaveanamespace"
				return recordLabelId
			},
			errContains: "cannot have a namespace",
		},
		"cluster scope with non-empty partition": {
			modFn: func(_, _, executiveId *pbresource.ID) *pbresource.ID {
				executiveId.Tenancy = &pbresource.Tenancy{Partition: resource.DefaultPartitionName}
				return executiveId
			},
			errContains: "cannot have a partition",
		},
		"cluster scope with non-empty namespace": {
			modFn: func(_, _, executiveId *pbresource.ID) *pbresource.ID {
				executiveId.Tenancy = &pbresource.Tenancy{Namespace: resource.DefaultNamespaceName}
				return executiveId
			},
			errContains: "cannot have a namespace",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
			require.NoError(t, err)

			executive, err := demo.GenerateV1Executive("music-man", "CEO")
			require.NoError(t, err)

			// Each test case picks which resource to use based on the resource type's scope.
			req := &pbresource.ReadRequest{Id: tc.modFn(artist.Id, recordLabel.Id, executive.Id)}

			_, err = client.Read(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestRead_TypeNotFound(t *testing.T) {
	server := svc.NewServer(svc.Config{Registry: resource.NewRegistry()})
	client := testClient(t, server)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Read(context.Background(), &pbresource.ReadRequest{Id: artist.Id})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.Artist not registered")
}

func TestRead_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			_, err = client.Write(tc.ctx, &pbresource.WriteRequest{Resource: artist})
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
			for tenancyDesc, modFn := range tenancyCases() {
				t.Run(tenancyDesc, func(t *testing.T) {
					client := svctest.NewResourceServiceBuilder().
						WithRegisterFns(demo.RegisterTypes).
						Run(t)

					recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
					require.NoError(t, err)
					rsp1, err := client.Write(tc.ctx, &pbresource.WriteRequest{Resource: recordLabel})
					recordLabel = rsp1.Resource
					require.NoError(t, err)

					artist, err := demo.GenerateV2Artist()
					require.NoError(t, err)
					rsp2, err := client.Write(tc.ctx, &pbresource.WriteRequest{Resource: artist})
					artist = rsp2.Resource
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
			mockBackend := svc.NewMockBackend(t)
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
		res          *pbresource.Resource
		authz        resolver.Result
		codeNotExist codes.Code
		codeExists   codes.Code
	}

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	label, err := demo.GenerateV1RecordLabel("blink1982")
	require.NoError(t, err)

	testcases := map[string]testCase{
		"artist-v1/read hook denied": {
			res:          artist,
			authz:        AuthorizerFrom(t, demo.ArtistV1ReadPolicy),
			codeNotExist: codes.PermissionDenied,
			codeExists:   codes.PermissionDenied,
		},
		"artist-v2/read hook allowed": {
			res:          artist,
			authz:        AuthorizerFrom(t, demo.ArtistV2ReadPolicy),
			codeNotExist: codes.NotFound,
			codeExists:   codes.OK,
		},
		// Labels have the read ACL that requires reading the data.
		"label-v1/read hook denied": {
			res:          label,
			authz:        AuthorizerFrom(t, demo.LabelV1ReadPolicy),
			codeNotExist: codes.NotFound,
			codeExists:   codes.PermissionDenied,
		},
	}

	adminAuthz := AuthorizerFrom(t, `key_prefix "" { policy = "write" }`)

	idx := 0
	nextTokenContext := func(t *testing.T) context.Context {
		// Each query should use a distinct token string to avoid caching so we can
		// change the behavior each call.
		token := fmt.Sprintf("token-%d", idx)
		idx++
		//nolint:staticcheck
		return context.WithValue(testContext(t), "x-consul-token", token)
	}

	for desc, tc := range testcases {
		t.Run(desc, func(t *testing.T) {
			dr := &dummyACLResolver{
				result: testutils.ACLsDisabled(t),
			}

			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				WithACLResolver(dr).
				Run(t)

			dr.SetResult(tc.authz)
			testutil.RunStep(t, "does not exist", func(t *testing.T) {
				_, err = client.Read(nextTokenContext(t), &pbresource.ReadRequest{Id: tc.res.Id})
				if tc.codeNotExist == codes.OK {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
				require.Equal(t, tc.codeNotExist.String(), status.Code(err).String(), "%v", err)
			})

			// Create it.
			dr.SetResult(adminAuthz)
			_, err = client.Write(nextTokenContext(t), &pbresource.WriteRequest{Resource: tc.res})
			require.NoError(t, err, "could not write resource")

			dr.SetResult(tc.authz)
			testutil.RunStep(t, "does exist", func(t *testing.T) {
				// exercise ACL when the data does exist
				_, err = client.Read(nextTokenContext(t), &pbresource.ReadRequest{Id: tc.res.Id})
				if tc.codeExists == codes.OK {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
				require.Equal(t, tc.codeExists.String(), status.Code(err).String())
			})
		})
	}
}

type dummyACLResolver struct {
	lock   sync.Mutex
	result resolver.Result
}

var _ svc.ACLResolver = (*dummyACLResolver)(nil)

func (r *dummyACLResolver) SetResult(result resolver.Result) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.result = result
}

func (r *dummyACLResolver) ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (resolver.Result, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.result, nil
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
