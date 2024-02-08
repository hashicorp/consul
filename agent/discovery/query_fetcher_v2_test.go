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
	mockpbresource "github.com/hashicorp/consul/grpcmocks/proto-public/pbresource/v1"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	"github.com/hashicorp/consul/sdk/testutil"
)

var (
	unknownErr = errors.New("I don't feel so good")
)

// Test_FetchService tests the FetchService method in scenarios where the RPC
// call succeeds and fails.
func Test_FetchWorkload(t *testing.T) {

	rc := &config.RuntimeConfig{
		DNSOnlyPassing: false,
	}

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
				Node: &Location{Name: "foo-1234", Address: "1.2.3.4"},
				Type: ResultTypeWorkload,
				Tenancy: ResultTenancy{
					Namespace: resource.DefaultNamespaceName,
					Partition: resource.DefaultPartitionName,
				},
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
				Node:       &Location{Name: "foo-1234", Address: "1.2.3.4"},
				Type:       ResultTypeWorkload,
				PortName:   "api",
				PortNumber: 5678,
				Tenancy: ResultTenancy{
					Namespace: resource.DefaultNamespaceName,
					Partition: resource.DefaultPartitionName,
				},
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
				Node: &Location{Name: "foo-1234", Address: "1.2.3.4"},
				Type: ResultTypeWorkload,
				Tenancy: ResultTenancy{
					Namespace: "test-namespace",
					Partition: "test-partition",
				},
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

// Test_V2FetchEndpoints the FetchService method in scenarios where the RPC
// call succeeds and fails.
func Test_V2FetchEndpoints(t *testing.T) {

	tests := []struct {
		name                string
		queryPayload        *QueryPayload
		context             Context
		configureMockClient func(mockClient *mockpbresource.ResourceServiceClient_Expecter)
		rc                  *config.RuntimeConfig
		expectedResult      []*Result
		expectedErr         error
		verifyShuffle       bool
	}{
		{
			name: "FetchEndpoints returns result",
			queryPayload: &QueryPayload{
				Name: "consul",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				results := []*pbcatalog.Endpoint{
					makeEndpoint("consul-1", "1.2.3.4", pbcatalog.Health_HEALTH_PASSING, 0, 0),
				}

				result := getTestEndpointsResponse(t, "", "", results...)
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: []*Result{
				{
					Node: &Location{Name: "consul-1", Address: "1.2.3.4"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
			},
		},
		{
			name: "FetchEndpoints returns empty result with no endpoints",
			queryPayload: &QueryPayload{
				Name: "consul",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {

				result := getTestEndpointsResponse(t, "", "")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: []*Result{},
		},
		{
			name: "FetchEndpoints returns a name error when the ServiceEndpoint does not exist",
			queryPayload: &QueryPayload{
				Name: "consul",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {

				result := getTestEndpointsResponse(t, "", "")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(nil, status.Error(codes.NotFound, "not found")).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedErr: ErrNotFound,
		},
		{
			name: "FetchEndpoints encounters a resource client error",
			queryPayload: &QueryPayload{
				Name: "consul",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {

				result := getTestEndpointsResponse(t, "", "")
				mockClient.Read(mock.Anything, mock.Anything).
					Return(nil, unknownErr).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedErr: unknownErr,
		},
		{
			name: "FetchEndpoints always filters out critical endpoints; DNS weights applied correctly",
			queryPayload: &QueryPayload{
				Name: "consul",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				results := []*pbcatalog.Endpoint{
					makeEndpoint("consul-1", "1.2.3.4", pbcatalog.Health_HEALTH_PASSING, 2, 3),
					makeEndpoint("consul-2", "2.3.4.5", pbcatalog.Health_HEALTH_WARNING, 2, 3),
					makeEndpoint("consul-3", "3.4.5.6", pbcatalog.Health_HEALTH_CRITICAL, 2, 3),
				}

				result := getTestEndpointsResponse(t, "", "", results...)
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: []*Result{
				{
					Node: &Location{Name: "consul-1", Address: "1.2.3.4"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 2,
				},
				{
					Node: &Location{Name: "consul-2", Address: "2.3.4.5"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 3,
				},
			},
		},
		{
			name: "FetchEndpoints filters out warning endpoints when DNSOnlyPassing is true",
			queryPayload: &QueryPayload{
				Name: "consul",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				results := []*pbcatalog.Endpoint{
					makeEndpoint("consul-1", "1.2.3.4", pbcatalog.Health_HEALTH_PASSING, 2, 3),
					makeEndpoint("consul-2", "2.3.4.5", pbcatalog.Health_HEALTH_WARNING, 2, 3),
					makeEndpoint("consul-3", "3.4.5.6", pbcatalog.Health_HEALTH_CRITICAL, 2, 3),
				}

				result := getTestEndpointsResponse(t, "", "", results...)
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			rc: &config.RuntimeConfig{
				DNSOnlyPassing: true,
			},
			expectedResult: []*Result{
				{
					Node: &Location{Name: "consul-1", Address: "1.2.3.4"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 2,
				},
			},
		},
		{
			name: "FetchEndpoints shuffles the results",
			queryPayload: &QueryPayload{
				Name: "consul",
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				results := []*pbcatalog.Endpoint{
					// use a set of 10 elements, the odds of getting the same result are 1 in 3628800
					makeEndpoint("consul-1", "10.0.0.1", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-2", "10.0.0.2", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-3", "10.0.0.3", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-4", "10.0.0.4", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-5", "10.0.0.5", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-6", "10.0.0.6", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-7", "10.0.0.7", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-8", "10.0.0.8", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-9", "10.0.0.9", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-10", "10.0.0.10", pbcatalog.Health_HEALTH_PASSING, 0, 0),
				}

				result := getTestEndpointsResponse(t, "", "", results...)
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: []*Result{
				{
					Node: &Location{Name: "consul-1", Address: "10.0.0.1"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-2", Address: "10.0.0.2"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-3", Address: "10.0.0.3"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-4", Address: "10.0.0.4"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-5", Address: "10.0.0.5"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-6", Address: "10.0.0.6"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-7", Address: "10.0.0.7"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-8", Address: "10.0.0.8"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-9", Address: "10.0.0.9"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
				{
					Node: &Location{Name: "consul-10", Address: "10.0.0.10"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
			},
			verifyShuffle: true,
		},
		{
			name: "FetchEndpoints returns only the specified limit",
			queryPayload: &QueryPayload{
				Name:  "consul",
				Limit: 1,
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				results := []*pbcatalog.Endpoint{
					// intentionally all the same to make this easier to verify
					makeEndpoint("consul-1", "10.0.0.1", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-1", "10.0.0.1", pbcatalog.Health_HEALTH_PASSING, 0, 0),
					makeEndpoint("consul-1", "10.0.0.1", pbcatalog.Health_HEALTH_PASSING, 0, 0),
				}

				result := getTestEndpointsResponse(t, "", "", results...)
				mockClient.Read(mock.Anything, mock.Anything).
					Return(result, nil).
					Once().
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*pbresource.ReadRequest)
						require.Equal(t, result.GetResource().GetId().GetName(), req.Id.Name)
					})
			},
			expectedResult: []*Result{
				{
					Node: &Location{Name: "consul-1", Address: "10.0.0.1"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: resource.DefaultNamespaceName,
						Partition: resource.DefaultPartitionName,
					},
					Weight: 1,
				},
			},
		},
		{
			name: "FetchEndpoints returns results with non-default tenancy",
			queryPayload: &QueryPayload{
				Name: "consul",
				Tenancy: QueryTenancy{
					Namespace: "test-namespace",
					Partition: "test-partition",
				},
			},
			context: Context{
				Token: "test-token",
			},
			configureMockClient: func(mockClient *mockpbresource.ResourceServiceClient_Expecter) {
				results := []*pbcatalog.Endpoint{
					// intentionally all the same to make this easier to verify
					makeEndpoint("consul-1", "10.0.0.1", pbcatalog.Health_HEALTH_PASSING, 0, 0),
				}

				result := getTestEndpointsResponse(t, "test-namespace", "test-partition", results...)
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
			expectedResult: []*Result{
				{
					Node: &Location{Name: "consul-1", Address: "10.0.0.1"},
					Type: ResultTypeWorkload,
					Tenancy: ResultTenancy{
						Namespace: "test-namespace",
						Partition: "test-partition",
					},
					Weight: 1,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := testutil.Logger(t)

			client := mockpbresource.NewResourceServiceClient(t)
			mockClient := client.EXPECT()
			tc.configureMockClient(mockClient)

			if tc.rc == nil {
				tc.rc = &config.RuntimeConfig{
					DNSOnlyPassing: false,
				}
			}

			df := NewV2DataFetcher(tc.rc, client, logger)

			result, err := df.FetchEndpoints(tc.context, tc.queryPayload, LookupTypeService)
			require.True(t, errors.Is(err, tc.expectedErr))

			if tc.verifyShuffle {
				require.NotEqualf(t, tc.expectedResult, result, "expected result to be shuffled. There is a small probability that it shuffled back to the original order. In that case, you may want to play the lottery.")
			}

			require.ElementsMatchf(t, tc.expectedResult, result, "elements of results should match")
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
				Tenancy: resource.DefaultNamespacedTenancy(),
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

func makeEndpoint(name string, address string, health pbcatalog.Health, weightPassing, weightWarning uint32) *pbcatalog.Endpoint {
	endpoint := &pbcatalog.Endpoint{
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: address,
			},
		},
		HealthStatus: health,
		TargetRef: &pbresource.ID{
			Name: name,
		},
	}

	if weightPassing > 0 || weightWarning > 0 {
		endpoint.Dns = &pbcatalog.DNSPolicy{
			Weights: &pbcatalog.Weights{
				Passing: weightPassing,
				Warning: weightWarning,
			},
		}
	}

	return endpoint
}

func getTestEndpointsResponse(t *testing.T, nsOverride string, partitionOverride string, endpoints ...*pbcatalog.Endpoint) *pbresource.ReadResponse {
	serviceEndpoints := &pbcatalog.ServiceEndpoints{
		Endpoints: endpoints,
	}

	data, err := anypb.New(serviceEndpoints)
	require.NoError(t, err)

	resp := &pbresource.ReadResponse{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    "consul",
				Type:    pbcatalog.ServiceType,
				Tenancy: resource.DefaultNamespacedTenancy(),
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
