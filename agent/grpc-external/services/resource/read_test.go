// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestRead_TypeNotFound(t *testing.T) {
	server := NewServer(Config{Registry: resource.NewRegistry()})
	client := testClient(t, server)

	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Read(context.Background(), &pbresource.ReadRequest{Id: artist.Id})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo/v2/artist not registered")
}

func TestRead_ResourceNotFound(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			demo.Register(server.Registry)
			client := testClient(t, server)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			_, err = client.Read(tc.ctx, &pbresource.ReadRequest{Id: artist.Id})
			require.Error(t, err)
			require.Equal(t, codes.NotFound.String(), status.Code(err).String())
			require.Contains(t, err.Error(), "resource not found")
		})
	}
}

func TestRead_GroupVersionMismatch(t *testing.T) {
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			demo.Register(server.Registry)
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
			server := testServer(t)
			demo.Register(server.Registry)
			client := testClient(t, server)

			artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			resource1, err := server.Backend.WriteCAS(tc.ctx, artist)
			require.NoError(t, err)

			rsp, err := client.Read(tc.ctx, &pbresource.ReadRequest{Id: artist.Id})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, resource1, rsp.Resource)
		})
	}
}

func TestRead_VerifyReadConsistencyArg(t *testing.T) {
	// Uses a mockBackend instead of the inmem Backend to verify the ReadConsistency argument is set correctly.
	for desc, tc := range readTestCases() {
		t.Run(desc, func(t *testing.T) {
			mockBackend := NewMockBackend(t)
			server := NewServer(Config{
				Registry: resource.NewRegistry(),
				Backend:  mockBackend,
			})
			demo.Register(server.Registry)

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
