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

func TestConnectCARoot(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &ConnectCARoot{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedCARoots
	rpc.On("RPC", mock.Anything, "ConnectCA.Roots", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*structs.DCSpecificRequest)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)

			reply := args.Get(3).(*structs.IndexedCARoots)
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	result, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.DCSpecificRequest{Datacenter: "dc1"})
	require.Nil(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestConnectCARoot_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &ConnectCARoot{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "wrong type")

}
