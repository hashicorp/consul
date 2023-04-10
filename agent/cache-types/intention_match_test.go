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

func TestIntentionMatch(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &IntentionMatch{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedIntentionMatches
	rpc.On("RPC", mock.Anything, "Intention.Match", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*structs.IntentionQueryRequest)
			require.Equal(t, uint64(24), req.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.MaxQueryTime)

			reply := args.Get(3).(*structs.IndexedIntentionMatches)
			reply.Index = 48
			resp = reply
		})

	// Fetch
	result, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.IntentionQueryRequest{Datacenter: "dc1"})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestIntentionMatch_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &IntentionMatch{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")

}
