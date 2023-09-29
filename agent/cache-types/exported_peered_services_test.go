package cachetype

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

func TestExportedPeeredServices(t *testing.T) {
	rpc := TestRPC(t)
	typ := &ExportedPeeredServices{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedExportedServiceList
	rpc.On("RPC", mock.Anything, "Internal.ExportedPeeredServices", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*structs.DCSpecificRequest)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)

			reply := args.Get(3).(*structs.IndexedExportedServiceList)
			reply.Services = map[string]structs.ServiceList{
				"my-peer": {
					structs.ServiceName{
						Name: "foo",
					},
					structs.ServiceName{
						Name: "bar",
					},
				},
			}
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.DCSpecificRequest{
		Datacenter: "dc1",
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)

	rpc.AssertExpectations(t)
}

func TestExportedPeeredServices_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	typ := &ExportedPeeredServices{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
	rpc.AssertExpectations(t)
}
