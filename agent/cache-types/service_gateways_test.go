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

func TestServiceGateways(t *testing.T) {
	rpc := TestRPC(t)
	typ := &ServiceGateways{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedCheckServiceNodes
	rpc.On("RPC", mock.Anything, "Internal.ServiceGateways", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*structs.ServiceSpecificRequest)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)
			require.Equal(t, "foo", req.ServiceName)

			nodes := []structs.CheckServiceNode{
				{
					Service: &structs.NodeService{
						Tags: req.ServiceTags,
					},
				},
			}

			reply := args.Get(3).(*structs.IndexedCheckServiceNodes)
			reply.Nodes = nodes
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "foo",
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)

	rpc.AssertExpectations(t)
}
