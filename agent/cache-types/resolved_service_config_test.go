package cachetype

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestResolvedServiceConfig(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &ResolvedServiceConfig{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.ServiceConfigResponse
	rpc.On("RPC", "ConfigEntry.ResolveServiceConfig", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.ServiceConfigRequest)
			require.Equal(uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(1*time.Second, req.QueryOptions.MaxQueryTime)
			require.Equal("foo", req.Name)
			require.True(req.AllowStale)

			reply := args.Get(2).(*structs.ServiceConfigResponse)
			reply.ProxyConfig = map[string]interface{}{
				"protocol": "http",
			}
			reply.UpstreamConfigs = map[string]map[string]interface{}{
				"s2": map[string]interface{}{
					"protocol": "http",
				},
			}

			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.ServiceConfigRequest{
		Datacenter: "dc1",
		Name:       "foo",
	})
	require.NoError(err)
	require.Equal(cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)
}

func TestResolvedServiceConfig_badReqType(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &ResolvedServiceConfig{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(err)
	require.Contains(err.Error(), "wrong type")

}
