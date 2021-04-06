package cachetype

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIntentionUpstreams(t *testing.T) {
	rpc := TestRPC(t)
	typ := &IntentionUpstreams{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedServiceList
	rpc.On("RPC", "Internal.IntentionUpstreams", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.ServiceSpecificRequest)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)
			require.Equal(t, "foo", req.ServiceName)

			services := structs.ServiceList{
				{Name: "foo"},
			}
			reply := args.Get(2).(*structs.IndexedServiceList)
			reply.Services = services
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
