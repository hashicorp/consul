// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/config"
	mockpbresource "github.com/hashicorp/consul/grpcmocks/proto-public/pbresource"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

// Test_FetchService tests the FetchService method in scenarios where the RPC
// call succeeds and fails.
func Test_FetchWorkload(t *testing.T) {

	rc := &config.RuntimeConfig{
		DNSOnlyPassing: false,
	}

	unknownErr := errors.New("I don't feel so good")

	tests := []struct {
		name                string
		queryPayload        *QueryPayload
		context             Context
		configureMockClient func(mockClient *mockpbresource.ResourceServiceClient_Expecter)
		expectedResult      *Result
		expectedErr         error
	}{
		{
			name: "FetchWorkload returns result",
			queryPayload: &QueryPayload{
				Name: "foo-1234",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				result := getTestWorkloadResponse(t, "", "")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: &Result{
				Address: "1.2.3.4",
				Type:    ResultTypeWorkload,
				Tenancy: ResultTenancy{
					Namespace: resource.DefaultNamespaceName,
					Partition: resource.DefaultPartitionName,
				},
				Target: "foo-1234",
			},
			expectedErr: nil,
		},
		{
			name: "FetchWorkload for non-existent workload",
			queryPayload: &QueryPayload{
				Name: "foo-1234",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				input := getTestWorkloadResponse(t, "", "")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(nil, status.Error(codes.NotFound, "not found")).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, input.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: nil,
			expectedErr:    ErrNotFound,
		},
		{
			name: "FetchWorkload encounters a resource client error",
			queryPayload: &QueryPayload{
				Name: "foo-1234",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				input := getTestWorkloadResponse(t, "", "")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(nil, unknownErr).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, input.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: nil,
			expectedErr:    unknownErr,
		},
		{
			name: "FetchWorkload with a matching port",
			queryPayload: &QueryPayload{
				Name:     "foo-1234",
				PortName: "api",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				result := getTestWorkloadResponse(t, "", "")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: &Result{
				Address:    "1.2.3.4",
				Type:       ResultTypeWorkload,
				PortName:   "api",
				PortNumber: 5678,
				Tenancy: ResultTenancy{
					Namespace: resource.DefaultNamespaceName,
					Partition: resource.DefaultPartitionName,
				},
				Target: "foo-1234",
			},
			expectedErr: nil,
		},
		{
			name: "FetchWorkload with a matching port",
			queryPayload: &QueryPayload{
				Name:     "foo-1234",
				PortName: "not-api",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				result := getTestWorkloadResponse(t, "", "")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: nil,
			expectedErr:    ErrNotFound,
		},
		{
			name: "FetchWorkload returns result for non-default tenancy",
			queryPayload: &QueryPayload{
				Name: "foo-1234",
				Tenancy: QueryTenancy{
					Namespace: "test-namespace",
					Partition: "test-partition",
				},
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				result := getTestWorkloadResponse(t, "test-namespace", "test-partition")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
						require.Equal(t, result.GetResource().GetId().GetTenancy().GetNamespace(), req.Id.Tenancy.Namespace)
						require.Equal(t, result.GetResource().GetId().GetTenancy().GetPartition(), req.Id.Tenancy.Partition)
					})
			},
			expectedResult: &Result{
				Address: "1.2.3.4",
				Type:    ResultTypeWorkload,
				Tenancy: ResultTenancy{
					Namespace: "test-namespace",
					Partition: "test-partition",
				},
				Target: "foo-1234",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := testutil.Logger(t)

			client := mockpbresource.NewResourceServiceClient(t)
			mockClient := client.EXPECT()
			tc.configureMockClient(mockClient)

			df := NewV2DataFetcher(rc, client, logger)

			result, err := df.FetchWorkload(tc.context, tc.queryPayload)
			require.True(t, errors.Is(err, tc.expectedErr))
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

func getTestWorkloadResponse(t *testing.T, nsOverride string, partitionOverride string) *pbresource.ReadResponse {
	workload := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host:  "1.2.3.4",
				Ports: []string{"api"},
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"api": {
				Port: 5678,
			},
		},
		Identity: "test-identity",
	}

	data, err := anypb.New(workload)
	require.NoError(t, err)

	resp := &pbresource.ReadResponse{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    "foo-1234",
				Type:    pbcatalog.WorkloadType,
				Tenancy: resource.DefaultNamespacedTenancy(), // TODO (v2-dns): tenancy
			},
			Data: data,
		},
	}

	if nsOverride != "" {
		resp.Resource.Id.Tenancy.Namespace = nsOverride
	}
	if partitionOverride != "" {
		resp.Resource.Id.Tenancy.Partition = partitionOverride
	}

	return resp
}
