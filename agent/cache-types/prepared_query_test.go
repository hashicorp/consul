package cachetype

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

func TestPreparedQuery(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &PreparedQuery{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.PreparedQueryExecuteResponse
	rpc.On("RPC", mock.Anything, "PreparedQuery.Execute", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*structs.PreparedQueryExecuteRequest)
			require.Equal(t, "geo-db", req.QueryIDOrName)
			require.Equal(t, 10, req.Limit)
			require.True(t, req.AllowStale)

			reply := args.Get(3).(*structs.PreparedQueryExecuteResponse)
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	result, err := typ.Fetch(cache.FetchOptions{}, &structs.PreparedQueryExecuteRequest{
		Datacenter:    "dc1",
		QueryIDOrName: "geo-db",
		Limit:         10,
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestPreparedQuery_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &PreparedQuery{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
}
