// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestList_TypeNotFound(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	_, err := client.List(context.Background(), &pbresource.ListRequest{
		Type:       typev1,
		Tenancy:    tenancy,
		NamePrefix: "",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type mesh/v1/service not registered")
}

func TestList_Empty(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			client := testClient(t, server)
			server.registry.Register(resource.Registration{Type: typev1})

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       typev1,
				Tenancy:    tenancy,
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
			server := testServer(t)
			client := testClient(t, server)
			server.registry.Register(resource.Registration{Type: typev1})

			resources := make([]*pbresource.Resource, 10)
			for i := 0; i < len(resources); i++ {
				r := &pbresource.Resource{
					Id: &pbresource.ID{
						Uid:     fmt.Sprintf("uid%d", i),
						Name:    fmt.Sprintf("name%d", i),
						Type:    typev1,
						Tenancy: tenancy,
					},
					Version: "",
				}
				server.Backend.WriteCAS(tc.ctx, r)
			}

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       typev1,
				Tenancy:    tenancy,
				NamePrefix: "",
			})
			require.NoError(t, err)
			prototest.AssertElementsMatch(t, resources, rsp.Resources)
		})
	}
}

func TestList_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range listTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			client := testClient(t, server)
			server.registry.Register(resource.Registration{Type: typev1})
			server.Backend.WriteCAS(tc.ctx, &pbresource.Resource{Id: id2})

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{
				Type:       typev1,
				Tenancy:    tenancy,
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
			mockBackend := NewMockBackend(t)
			server := NewServer(Config{
				registry: resource.NewRegistry(),
				Backend:  mockBackend,
			})
			server.registry.Register(resource.Registration{Type: typev1})
			resource1 := &pbresource.Resource{Id: id1, Version: "1"}
			mockBackend.On("List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]*pbresource.Resource{resource1}, nil)
			client := testClient(t, server)

			rsp, err := client.List(tc.ctx, &pbresource.ListRequest{Type: typev1, Tenancy: tenancy, NamePrefix: ""})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, resource1, rsp.Resources[0])
			mockBackend.AssertCalled(t, "List", mock.Anything, tc.consistency, mock.Anything, mock.Anything, mock.Anything)
		})
	}
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
