package cachetype

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHealthServices(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &HealthServices{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedCheckServiceNodes
	rpc.On("RPC", "Health.ServiceNodes", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.ServiceSpecificRequest)
			require.Equal(uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(1*time.Second, req.QueryOptions.MaxQueryTime)
			require.Equal("web", req.ServiceName)
			require.True(req.AllowStale)

			reply := args.Get(2).(*structs.IndexedCheckServiceNodes)
			reply.Nodes = []structs.CheckServiceNode{
				{Service: &structs.NodeService{Tags: req.ServiceTags}},
			}
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
		ServiceTags: []string{"tag1", "tag2"},
	})
	require.NoError(err)
	require.Equal(cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)
}

func TestHealthServices_badReqType(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &HealthServices{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(err)
	require.Contains(err.Error(), "wrong type")

}
