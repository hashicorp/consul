package cachetype

import (
	"testing"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPreparedQuery(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &PreparedQuery{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.PreparedQueryExecuteResponse
	rpc.On("RPC", "PreparedQuery.Execute", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.PreparedQueryExecuteRequest)
			require.Equal("geo-db", req.QueryIDOrName)
			require.Equal(10, req.Limit)
			require.True(req.AllowStale)

			reply := args.Get(2).(*structs.PreparedQueryExecuteResponse)
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	result, err := typ.Fetch(cache.FetchOptions{}, &structs.PreparedQueryExecuteRequest{
		Datacenter:    "dc1",
		QueryIDOrName: "geo-db",
		Limit:         10,
	})
	require.NoError(err)
	require.Equal(cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestPreparedQuery_badReqType(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &PreparedQuery{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(err)
	require.Contains(err.Error(), "wrong type")
}
