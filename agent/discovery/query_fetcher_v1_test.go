// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// Test_FetchVirtualIP tests the FetchVirtualIP method in scenarios where the RPC
// call succeeds and fails.
func Test_FetchVirtualIP(t *testing.T) {
	// set these to confirm that RPC call does not use them for this particular RPC
	rc := &config.RuntimeConfig{
		DNSAllowStale:  true,
		DNSMaxStale:    100,
		DNSUseCache:    true,
		DNSCacheMaxAge: 100,
	}
	tests := []struct {
		name           string
		queryPayload   *QueryPayload
		context        Context
		expectedResult *Result
		expectedErr    error
	}{
		{
			name: "FetchVirtualIP returns result",
			queryPayload: &QueryPayload{
				Name: "db",
				Tenancy: QueryTenancy{
					Peer:      "test-peer",
					Namespace: defaultTestNamespace,
					Partition: defaultTestPartition,
				},
			},
			context: Context{
				Token: "test-token",
			},
			expectedResult: &Result{
				Service: &Location{
					Name:    "db",
					Address: "192.168.10.10",
				},
				Type: ResultTypeVirtual,
			},
			expectedErr: nil,
		},
		{
			name: "FetchVirtualIP returns error",
			queryPayload: &QueryPayload{
				Name: "db",
				Tenancy: QueryTenancy{
					Peer:      "test-peer",
					Namespace: defaultTestNamespace,
					Partition: defaultTestPartition},
			},
			context: Context{
				Token: "test-token",
			},
			expectedResult: nil,
			expectedErr:    errors.New("test-error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := testutil.Logger(t)
			mockRPC := cachetype.NewMockRPC(t)
			mockRPC.On("RPC", mock.Anything, "Catalog.VirtualIPForService", mock.Anything, mock.Anything).
				Return(tc.expectedErr).
				Run(func(args mock.Arguments) {
					req := args.Get(2).(*structs.ServiceSpecificRequest)

					// validate RPC options are not set from config for the VirtuaLIPForService RPC
					require.False(t, req.AllowStale)
					require.Equal(t, time.Duration(0), req.MaxStaleDuration)
					require.False(t, req.UseCache)
					require.Equal(t, time.Duration(0), req.MaxAge)

					// validate RPC options are set correctly from the queryPayload and context
					require.Equal(t, tc.queryPayload.Tenancy.Peer, req.PeerName)
					require.Equal(t, tc.queryPayload.Tenancy.Namespace, req.EnterpriseMeta.NamespaceOrEmpty())
					require.Equal(t, tc.queryPayload.Tenancy.Partition, req.EnterpriseMeta.PartitionOrEmpty())
					require.Equal(t, tc.context.Token, req.QueryOptions.Token)

					if tc.expectedErr == nil {
						// set the out parameter to ensure that it is used to formulate the result.Address
						reply := args.Get(3).(*string)
						*reply = tc.expectedResult.Service.Address
					}
				})
			translateServicePortFunc := func(dc string, port int, taggedAddresses map[string]structs.ServiceAddress) int { return 0 }
			rpcFuncForServiceNodes := func(ctx context.Context, req structs.ServiceSpecificRequest) (structs.IndexedCheckServiceNodes, cache.ResultMeta, error) {
				return structs.IndexedCheckServiceNodes{}, cache.ResultMeta{}, nil
			}
			rpcFuncForSamenessGroup := func(ctx context.Context, req *structs.ConfigEntryQuery) (structs.SamenessGroupConfigEntry, cache.ResultMeta, error) {
				return structs.SamenessGroupConfigEntry{}, cache.ResultMeta{}, nil
			}
			getFromCacheFunc := func(ctx context.Context, t string, r cache.Request) (interface{}, cache.ResultMeta, error) {
				return nil, cache.ResultMeta{}, nil
			}

			df := NewV1DataFetcher(rc, acl.DefaultEnterpriseMeta(), getFromCacheFunc, mockRPC.RPC, rpcFuncForServiceNodes, rpcFuncForSamenessGroup, translateServicePortFunc, logger)

			result, err := df.FetchVirtualIP(tc.context, tc.queryPayload)
			require.Equal(t, tc.expectedErr, err)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

// Test_FetchEndpoints tests the FetchEndpoints method in scenarios where the RPC
// call succeeds and fails.
func Test_FetchEndpoints(t *testing.T) {
	// set these to confirm that RPC call does not use them for this particular RPC
	rc := &config.RuntimeConfig{
		Datacenter:     "dc2",
		DNSAllowStale:  true,
		DNSMaxStale:    100,
		DNSUseCache:    true,
		DNSCacheMaxAge: 100,
	}
	ctx := Context{
		Token: "test-token",
	}
	expectedResults := []*Result{
		{
			Node: &Location{
				Name:            "node-name",
				Address:         "node-address",
				TaggedAddresses: map[string]*TaggedAddress{},
			},
			Service: &Location{
				Name:            "service-name",
				Address:         "service-address",
				TaggedAddresses: map[string]*TaggedAddress{},
			},
			Type: ResultTypeService,
			DNS: DNSConfig{
				Weight: 1,
			},
			Ports: []Port{
				{
					Number: 0,
				},
			},
			Tenancy: ResultTenancy{
				PeerName: "test-peer",
			},
		},
	}

	logger := testutil.Logger(t)
	mockRPC := cachetype.NewMockRPC(t)
	translateServicePortFunc := func(dc string, port int, taggedAddresses map[string]structs.ServiceAddress) int { return 0 }
	rpcFuncForSamenessGroup := func(ctx context.Context, req *structs.ConfigEntryQuery) (structs.SamenessGroupConfigEntry, cache.ResultMeta, error) {
		return structs.SamenessGroupConfigEntry{}, cache.ResultMeta{}, nil
	}
	getFromCacheFunc := func(ctx context.Context, t string, r cache.Request) (interface{}, cache.ResultMeta, error) {
		return nil, cache.ResultMeta{}, nil
	}
	rpcFuncForServiceNodes := func(ctx context.Context, req structs.ServiceSpecificRequest) (structs.IndexedCheckServiceNodes, cache.ResultMeta, error) {
		return structs.IndexedCheckServiceNodes{
			Nodes: []structs.CheckServiceNode{
				{
					Node: &structs.Node{
						Address: "node-address",
						Node:    "node-name",
					},
					Service: &structs.NodeService{
						Address:  "service-address",
						Service:  "service-name",
						PeerName: "test-peer",
					},
				},
			},
		}, cache.ResultMeta{}, nil
	}
	queryPayload := &QueryPayload{
		Name: "service-name",
		Tenancy: QueryTenancy{
			Peer:      "test-peer",
			Namespace: defaultTestNamespace,
			Partition: defaultTestPartition,
		},
	}

	df := NewV1DataFetcher(rc, acl.DefaultEnterpriseMeta(), getFromCacheFunc, mockRPC.RPC, rpcFuncForServiceNodes, rpcFuncForSamenessGroup, translateServicePortFunc, logger)

	results, err := df.FetchEndpoints(ctx, queryPayload, LookupTypeService)
	require.NoError(t, err)
	require.Equal(t, expectedResults, results)
}
