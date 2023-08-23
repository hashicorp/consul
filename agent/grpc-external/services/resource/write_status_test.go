// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestWriteStatus_ACL(t *testing.T) {
	type testCase struct {
		authz       resolver.Result
		assertErrFn func(error)
	}
	testcases := map[string]testCase{
		"denied": {
			authz: AuthorizerFrom(t, demo.ArtistV2WritePolicy),
			assertErrFn: func(err error) {
				require.Error(t, err)
				require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
			},
		},
		"allowed": {
			authz: AuthorizerFrom(t, demo.ArtistV2WritePolicy, `operator = "write"`),
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

			rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
			require.NoError(t, err)
			artist = rsp.Resource

			// exercise ACL
			_, err = client.WriteStatus(testContext(t), validWriteStatusRequest(t, artist))
			tc.assertErrFn(err)
		})
	}
}

func TestWriteStatus_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	testCases := map[string]func(*pbresource.WriteStatusRequest){
		"no id":                   func(req *pbresource.WriteStatusRequest) { req.Id = nil },
		"no type":                 func(req *pbresource.WriteStatusRequest) { req.Id.Type = nil },
		"no tenancy":              func(req *pbresource.WriteStatusRequest) { req.Id.Tenancy = nil },
		"no name":                 func(req *pbresource.WriteStatusRequest) { req.Id.Name = "" },
		"no uid":                  func(req *pbresource.WriteStatusRequest) { req.Id.Uid = "" },
		"no key":                  func(req *pbresource.WriteStatusRequest) { req.Key = "" },
		"no status":               func(req *pbresource.WriteStatusRequest) { req.Status = nil },
		"no observed generation":  func(req *pbresource.WriteStatusRequest) { req.Status.ObservedGeneration = "" },
		"bad observed generation": func(req *pbresource.WriteStatusRequest) { req.Status.ObservedGeneration = "bogus" },
		"no condition type":       func(req *pbresource.WriteStatusRequest) { req.Status.Conditions[0].Type = "" },
		"no reference type":       func(req *pbresource.WriteStatusRequest) { req.Status.Conditions[0].Resource.Type = nil },
		"no reference tenancy":    func(req *pbresource.WriteStatusRequest) { req.Status.Conditions[0].Resource.Tenancy = nil },
		"no reference name":       func(req *pbresource.WriteStatusRequest) { req.Status.Conditions[0].Resource.Name = "" },
		"updated at provided":     func(req *pbresource.WriteStatusRequest) { req.Status.UpdatedAt = timestamppb.Now() },
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			res, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			res.Id.Uid = ulid.Make().String()
			res.Generation = ulid.Make().String()

			req := validWriteStatusRequest(t, res)
			modFn(req)

			_, err = client.WriteStatus(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		})
	}
}

func TestWriteStatus_Success(t *testing.T) {
	for desc, fn := range map[string]func(*pbresource.WriteStatusRequest){
		"CAS":     func(*pbresource.WriteStatusRequest) {},
		"Non CAS": func(req *pbresource.WriteStatusRequest) { req.Version = "" },
	} {
		t.Run(desc, func(t *testing.T) {
			server := testServer(t)
			client := testClient(t, server)

			demo.RegisterTypes(server.Registry)

			res, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			writeRsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
			require.NoError(t, err)
			res = writeRsp.Resource

			req := validWriteStatusRequest(t, res)
			fn(req)

			rsp, err := client.WriteStatus(testContext(t), req)
			require.NoError(t, err)
			res = rsp.Resource

			req = validWriteStatusRequest(t, res)
			req.Key = "consul.io/other-controller"
			fn(req)

			rsp, err = client.WriteStatus(testContext(t), req)
			require.NoError(t, err)

			require.Equal(t, rsp.Resource.Generation, res.Generation, "generation should not have changed")
			require.NotEqual(t, rsp.Resource.Version, res.Version, "version should have changed")
			require.Contains(t, rsp.Resource.Status, "consul.io/other-controller")
			require.Contains(t, rsp.Resource.Status, "consul.io/artist-controller")
			require.NotNil(t, rsp.Resource.Status["consul.io/artist-controller"].UpdatedAt)
		})
	}
}

func TestWriteStatus_CASFailure(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	res = rsp.Resource

	req := validWriteStatusRequest(t, res)
	req.Version = "nope"

	_, err = client.WriteStatus(testContext(t), req)
	require.Error(t, err)
	require.Equal(t, codes.Aborted.String(), status.Code(err).String())
}

func TestWriteStatus_TypeNotFound(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	res.Id.Uid = ulid.Make().String()
	res.Generation = ulid.Make().String()

	_, err = client.WriteStatus(testContext(t), validWriteStatusRequest(t, res))
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.Artist not registered")
}

func TestWriteStatus_ResourceNotFound(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	res.Id.Uid = ulid.Make().String()
	res.Generation = ulid.Make().String()

	_, err = client.WriteStatus(testContext(t), validWriteStatusRequest(t, res))
	require.Error(t, err)
	require.Equal(t, codes.NotFound.String(), status.Code(err).String())
}

func TestWriteStatus_WrongUid(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)
	demo.RegisterTypes(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	res = rsp.Resource

	req := validWriteStatusRequest(t, res)
	req.Id.Uid = ulid.Make().String()

	_, err = client.WriteStatus(testContext(t), req)
	require.Error(t, err)
	require.Equal(t, codes.NotFound.String(), status.Code(err).String())
}

func TestWriteStatus_NonCASUpdate_Retry(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.RegisterTypes(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	res = rsp.Resource

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
		req := validWriteStatusRequest(t, res)
		req.Version = ""

		_, err := client.WriteStatus(testContext(t), req)
		errCh <- err
	}()

	// Wait for the read, to ensure the Write in the goroutine above has read the
	// current version of the resource.
	<-backend.readCh

	// Update the resource.
	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: modifyArtist(t, res)})
	require.NoError(t, err)

	// Unblock the read.
	close(backend.blockCh)

	// Check that the write succeeded anyway because of a retry.
	require.NoError(t, <-errCh)
}

func validWriteStatusRequest(t *testing.T, res *pbresource.Resource) *pbresource.WriteStatusRequest {
	t.Helper()

	album, err := demo.GenerateV2Album(res.Id)
	require.NoError(t, err)

	return &pbresource.WriteStatusRequest{
		Id:      res.Id,
		Version: res.Version,
		Key:     "consul.io/artist-controller",
		Status: &pbresource.Status{
			ObservedGeneration: res.Generation,
			Conditions: []*pbresource.Condition{
				{
					Type:     "AlbumCreated",
					State:    pbresource.Condition_STATE_TRUE,
					Reason:   "AlbumCreated",
					Message:  fmt.Sprintf("Album '%s' created", album.Id.Name),
					Resource: resource.Reference(album.Id, ""),
				},
			},
		},
	}
}
