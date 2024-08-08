// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMutateAndValidate_InputValidation(t *testing.T) {
	run := func(t *testing.T, client pbresource.ResourceServiceClient, tc resourceValidTestCase) {
		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
		require.NoError(t, err)

		req := &pbresource.MutateAndValidateRequest{Resource: tc.modFn(artist, recordLabel)}
		_, err = client.MutateAndValidate(testContext(t), req)
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		require.ErrorContains(t, err, tc.errContains)
	}

	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	for desc, tc := range resourceValidTestCases(t) {
		t.Run(desc, func(t *testing.T) {
			run(t, client, tc)
		})
	}
}

func TestMutateAndValidate_OwnerValidation(t *testing.T) {
	run := func(t *testing.T, client pbresource.ResourceServiceClient, tc ownerValidTestCase) {
		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		album, err := demo.GenerateV2Album(artist.Id)
		require.NoError(t, err)

		tc.modFn(album)

		_, err = client.MutateAndValidate(testContext(t), &pbresource.MutateAndValidateRequest{Resource: album})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		require.ErrorContains(t, err, tc.errorContains)
	}

	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	for desc, tc := range ownerValidationTestCases(t) {
		t.Run(desc, func(t *testing.T) {
			run(t, client, tc)
		})
	}
}

func TestMutateAndValidate_TypeNotFound(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().Run(t)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.MutateAndValidate(testContext(t), &pbresource.MutateAndValidateRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.Artist not registered")
}

func TestMutateAndValidate_Success(t *testing.T) {
	run := func(t *testing.T, client pbresource.ResourceServiceClient, tc mavOrWriteSuccessTestCase) {
		recordLabel, err := demo.GenerateV1RecordLabel("looney-tunes")
		require.NoError(t, err)

		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.MutateAndValidate(testContext(t), &pbresource.MutateAndValidateRequest{Resource: tc.modFn(artist, recordLabel)})
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expectedTenancy, rsp.Resource.Id.Tenancy)
	}

	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	for desc, tc := range mavOrWriteSuccessTestCases(t) {
		t.Run(desc, func(t *testing.T) {
			run(t, client, tc)
		})
	}
}

func TestMutateAndValidate_Mutate(t *testing.T) {
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

	rsp, err := client.MutateAndValidate(testContext(t), &pbresource.MutateAndValidateRequest{Resource: artist})
	require.NoError(t, err)

	// verify mutate hook set genre to disco
	require.NoError(t, rsp.Resource.Data.UnmarshalTo(artistData))
	require.Equal(t, pbdemov2.Genre_GENRE_DISCO, artistData.Genre)
}

func TestMutateAndValidate_TenancyMarkedForDeletion_Fails(t *testing.T) {
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

			_, err = client.MutateAndValidate(testContext(t), &pbresource.MutateAndValidateRequest{Resource: tc.modFn(artist, recordLabel, mockTenancyBridge)})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), tc.errContains)
		})
	}
}
