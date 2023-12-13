// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

// TODO: Update all tests to use true/false table test for v2tenancy

func TestList_InputValidation(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	type testCase struct {
		modReqFn    func(req *pbresource.ListRequest)
		errContains string
	}

	testCases := map[string]testCase{
		"no type": {
			modReqFn:    func(req *pbresource.ListRequest) { req.Type = nil },
			errContains: "type is required",
		},
		"no tenancy": {
			modReqFn:    func(req *pbresource.ListRequest) { req.Tenancy = nil },
			errContains: "tenancy is required",
		},
		"partition mixed case": {
			modReqFn:    func(req *pbresource.ListRequest) { req.Tenancy.Partition = "Default" },
			errContains: "tenancy.partition invalid",
		},
		"partition too long": {
			modReqFn: func(req *pbresource.ListRequest) {
				req.Tenancy.Partition = strings.Repeat("p", resource.MaxNameLength+1)
			},
			errContains: "tenancy.partition invalid",
		},
		"namespace mixed case": {
			modReqFn:    func(req *pbresource.ListRequest) { req.Tenancy.Namespace = "Default" },
			errContains: "tenancy.namespace invalid",
		},
		"namespace too long": {
			modReqFn: func(req *pbresource.ListRequest) {
				req.Tenancy.Namespace = strings.Repeat("n", resource.MaxNameLength+1)
			},
			errContains: "tenancy.namespace invalid",
		},
		"name_prefix mixed case": {
			modReqFn:    func(req *pbresource.ListRequest) { req.NamePrefix = "Violator" },
			errContains: "name_prefix invalid",
		},
		"partitioned resource provides non-empty namespace": {
			modReqFn: func(req *pbresource.ListRequest) {
				req.Type = demo.TypeV1RecordLabel
				req.Tenancy.Namespace = "bad"
			},
			errContains: "cannot have a namespace",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			req := &pbresource.ListRequest{
				Type:    demo.TypeV2Album,
				Tenancy: resource.DefaultNamespacedTenancy(),
			}
			tc.modReqFn(req)

			_, err := client.List(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestList_TypeNotFound(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().Run(t)

	_, err := client.List(context.Background(), &pbresource.ListRequest{
		Type:       demo.TypeV2Artist,
		Tenancy:    resource.DefaultNamespacedTenancy(),
		NamePrefix: "",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.Artist not registered")
}

func TestList_Empty(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       demo.TypeV1Artist,
				Tenancy:    resource.DefaultNamespacedTenancy(),
				NamePrefix: "",
			})
			require.NoError(t, err)
			require.Empty(t, rsp.Resources)
		})
	}
}

func TestList_Many(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			resources := make([]*pbresource.Resource, 10)
			for i := 0; i < len(resources); i++ {
				artist, err := demo.GenerateV2Artist()
				require.NoError(t, err)

				// Prevent test flakes if the generated names collide.
				artist.Id.Name = fmt.Sprintf("%s-%d", artist.Id.Name, i)

				rsp, err := client.Write(tc.ctx, &pbresource.WriteRequest{Resource: artist})
				require.NoError(t, err)

				resources[i] = rsp.Resource
			}

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       demo.TypeV2Artist,
				Tenancy:    resource.DefaultNamespacedTenancy(),
				NamePrefix: "",
			})
			require.NoError(t, err)
			prototest.AssertElementsMatch(t, resources, rsp.Resources)
		})
	}
}

func TestList_NamePrefix(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			expectedResources := []*pbresource.Resource{}

			namePrefixIndex := 0
			// create a name prefix that is always present
			namePrefix := fmt.Sprintf("%s-", strconv.Itoa(namePrefixIndex))
			for i := 0; i < 10; i++ {
				artist, err := demo.GenerateV2Artist()
				require.NoError(t, err)

				// Prevent test flakes if the generated names collide.
				artist.Id.Name = fmt.Sprintf("%d-%s", i, artist.Id.Name)

				rsp, err := client.Write(tc.ctx, &pbresource.WriteRequest{Resource: artist})
				require.NoError(t, err)

				// only matching name prefix are expected
				if i == namePrefixIndex {
					expectedResources = append(expectedResources, rsp.Resource)
				}
			}

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       demo.TypeV2Artist,
				Tenancy:    resource.DefaultNamespacedTenancy(),
				NamePrefix: namePrefix,
			})

			require.NoError(t, err)
			prototest.AssertElementsMatch(t, expectedResources, rsp.Resources)
		})
	}
}

func TestList_Tenancy_Defaults_And_Normalization(t *testing.T) {
	// Test units of tenancy get defaulted correctly when empty.
	ctx := context.Background()
	for desc, tc := range wildcardTenancyCases() {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			// Write partition scoped record label
			recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
			require.NoError(t, err)
			recordLabelRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: recordLabel})
			require.NoError(t, err)

			// Write namespace scoped artist
			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)
			artistRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
			require.NoError(t, err)

			// Write a cluster scoped Executive
			executive, err := demo.GenerateV1Executive("king-arthur", "CEO")
			require.NoError(t, err)
			executiveRsp, err := client.Write(ctx, &pbresource.WriteRequest{Resource: executive})
			require.NoError(t, err)

			// List and verify correct resource returned for empty tenancy units.
			listRsp, err := client.List(ctx, &pbresource.ListRequest{
				Type:    tc.typ,
				Tenancy: tc.tenancy,
			})
			require.NoError(t, err)
			require.Len(t, listRsp.Resources, 1)
			switch tc.typ {
			case demo.TypeV1RecordLabel:
				prototest.AssertDeepEqual(t, recordLabelRsp.Resource, listRsp.Resources[0])
			case demo.TypeV1Artist:
				prototest.AssertDeepEqual(t, artistRsp.Resource, listRsp.Resources[0])
			case demo.TypeV1Executive:
				prototest.AssertDeepEqual(t, executiveRsp.Resource, listRsp.Resources[0])
			}
		})
	}
}

func TestList_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			client := svctest.NewResourceServiceBuilder().
				WithRegisterFns(demo.RegisterTypes).
				Run(t)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			_, err = client.Write(tc.ctx, &pbresource.WriteRequest{Resource: artist})
			require.NoError(t, err)

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       demo.TypeV1Artist,
				Tenancy:    artist.Id.Tenancy,
				NamePrefix: "",
			})
			require.NoError(t, err)
			require.Empty(t, rsp.Resources)
		})
	}
}

func TestList_VerifyReadConsistencyArg(t *testing.T) {
	// Uses a mockBackend instead of the inmem Backend to verify the ReadConsistency argument is set correctly.
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			mockBackend := svc.NewMockBackend(t)
			server := testServer(t)
			server.Backend = mockBackend
			demo.RegisterTypes(server.Registry)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			mockBackend.On("List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]*pbresource.Resource{artist}, nil)
			client := testClient(t, server)

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{Type: artist.Id.Type, Tenancy: artist.Id.Tenancy, NamePrefix: ""})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, artist, rsp.Resources[0])
			mockBackend.AssertCalled(t, "List", mock.Anything, tc.consistency, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestList_ACL_ListDenied(t *testing.T) {
	t.Parallel()

	// deny all
	_, _, err := roundTripList(t, testutils.ACLNoPermissions(t))

	// verify key:list denied
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "lacks permission 'key:list'")
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestList_ACL_ListAllowed_ReadDenied(t *testing.T) {
	t.Parallel()

	// allow list, deny read
	authz := AuthorizerFrom(t, demo.ArtistV2ListPolicy,
		`key_prefix "resource/demo.v2.Artist/" { policy = "deny" }`)
	_, rsp, err := roundTripList(t, authz)

	// verify resource filtered out by key:read denied hence no results
	require.NoError(t, err)
	require.Empty(t, rsp.Resources)
}

// N.B. Uses key ACLs for now. See demo.RegisterTypes()
func TestList_ACL_ListAllowed_ReadAllowed(t *testing.T) {
	t.Parallel()

	// allow list, allow read
	authz := AuthorizerFrom(t, demo.ArtistV2ListPolicy, demo.ArtistV2ReadPolicy)
	artist, rsp, err := roundTripList(t, authz)

	// verify resource not filtered out by acl
	require.NoError(t, err)
	require.Len(t, rsp.Resources, 1)
	prototest.AssertDeepEqual(t, artist, rsp.Resources[0])
}

func roundTripList(t *testing.T, authz acl.Authorizer) (*pbresource.Resource, *pbresource.ListResponse, error) {
	ctx := testContext(t)
	builder := svctest.NewResourceServiceBuilder().WithRegisterFns(demo.RegisterTypes)
	client := builder.Run(t)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(ctx, &pbresource.WriteRequest{Resource: artist})
	require.NoError(t, err)

	// Put ACLResolver in place after above writes so writes not subject to ACLs
	mockACLResolver := &svc.MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(authz, nil)
	builder.ServiceImpl().Config.ACLResolver = mockACLResolver

	rsp2, err := client.List(
		ctx,
		&pbresource.ListRequest{
			Type:       artist.Id.Type,
			Tenancy:    artist.Id.Tenancy,
			NamePrefix: "",
		},
	)
	return rsp1.Resource, rsp2, err
}

type listTestCase struct {
	consistency storage.ReadConsistency
	ctx         context.Context
}

func listTestCases() map[string]listTestCase {
	return map[string]listTestCase{
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
