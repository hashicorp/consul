package cachetype

import (
	"testing"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCatalogDatacenters(t *testing.T) {
	rpc := TestRPC(t)
	typ := &CatalogDatacenters{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *[]string
	var resp2 *[]string
	var resp3 *[]string
	rpc.On("RPC", "Catalog.ListDatacenters", mock.Anything, mock.Anything).Once().Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DatacentersRequest)
			require.True(t, req.AllowStale)

			reply := args.Get(2).(*[]string)
			*reply = []string{
				"primary", "secondary", "tertiary",
			}
			resp = reply
		})
	rpc.On("RPC", "Catalog.ListDatacenters", mock.Anything, mock.Anything).Once().Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DatacentersRequest)
			require.True(t, req.AllowStale)

			reply := args.Get(2).(*[]string)
			*reply = []string{
				"primary", "tertiary", "secondary",
			}
			resp2 = reply
		})
	rpc.On("RPC", "Catalog.ListDatacenters", mock.Anything, mock.Anything).Once().Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DatacentersRequest)
			require.True(t, req.AllowStale)

			reply := args.Get(2).(*[]string)
			*reply = []string{
				"primary", "secondary",
			}
			resp3 = reply
		})

	// Fetch first time
	result, err := typ.Fetch(cache.FetchOptions{}, &structs.DatacentersRequest{})
	result2, err := typ.Fetch(cache.FetchOptions{LastResult: &result}, &structs.DatacentersRequest{QueryOptions: structs.QueryOptions{MustRevalidate: true}})
	result3, err := typ.Fetch(cache.FetchOptions{LastResult: &result2}, &structs.DatacentersRequest{QueryOptions: structs.QueryOptions{MustRevalidate: true}})

	// make sure it was called the right number of times
	rpc.AssertExpectations(t)

	// make sure the first result was correct
	require.NoError(t, err)
	require.Equal(t, result, cache.FetchResult{
		Value: resp,
		Index: 1,
	})

	// validate the second result
	require.NoError(t, err)
	require.Equal(t, result2, cache.FetchResult{
		Value: resp2,
		Index: 2,
	})

	// validate the third result
	require.NoError(t, err)
	require.Equal(t, result3, cache.FetchResult{
		Value: resp3,
		Index: 3,
	})

}

func TestDatacenters_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	typ := &PreparedQuery{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
	rpc.AssertExpectations(t)
}
