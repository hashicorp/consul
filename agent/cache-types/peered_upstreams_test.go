package cachetype

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

func TestPeeredUpstreams(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &PeeredUpstreams{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedPeeredServiceList
	rpc.On("RPC", mock.Anything, "Internal.PeeredUpstreams", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(2).(*structs.PartitionSpecificRequest)
			require.Equal(t, uint64(24), req.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)

			reply := args.Get(3).(*structs.IndexedPeeredServiceList)
			reply.Index = 48
			resp = reply
		})

	// Fetch
	result, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.PartitionSpecificRequest{
		Datacenter:     "dc1",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestPeeredUpstreams_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &PeeredUpstreams{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
}
