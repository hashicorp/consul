// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cachetype

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

func TestNodeServices(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &NodeServices{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedNodeServices
	rpc.On("RPC", mock.Anything, "Catalog.NodeServices", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*structs.NodeSpecificRequest)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.Equal(t, "node-01", req.Node)
			require.True(t, req.AllowStale)

			reply := args.Get(3).(*structs.IndexedNodeServices)
			reply.NodeServices = &structs.NodeServices{
				Node: &structs.Node{
					ID:         "abcdef",
					Node:       "node-01",
					Address:    "127.0.0.5",
					Datacenter: "dc1",
				},
			}

			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "node-01",
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)
}

func TestNodeServices_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &NodeServices{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")

}
