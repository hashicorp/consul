package cachetype

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConfigEntries(t *testing.T) {
	rpc := TestRPC(t)
	typ := &ConfigEntries{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedConfigEntries
	rpc.On("RPC", "ConfigEntry.List", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.ConfigEntryQuery)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)
			require.Equal(t, structs.ServiceResolver, req.Kind)
			require.Equal(t, "", req.Name)

			reply := args.Get(2).(*structs.IndexedConfigEntries)
			reply.Kind = structs.ServiceResolver
			reply.Entries = []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{Kind: structs.ServiceResolver, Name: "foo"},
				&structs.ServiceResolverConfigEntry{Kind: structs.ServiceResolver, Name: "bar"},
			}
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.ConfigEntryQuery{
		Datacenter: "dc1",
		Kind:       structs.ServiceResolver,
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)

	rpc.AssertExpectations(t)
}

func TestConfigEntries_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	typ := &ConfigEntries{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
	rpc.AssertExpectations(t)
}
