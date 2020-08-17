package cachetype

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConnectCARoot(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &ConnectCARoot{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedCARoots
	rpc.On("RPC", "ConnectCA.Roots", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DCSpecificRequest)
			require.Equal(uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(1*time.Second, req.QueryOptions.MaxQueryTime)

			reply := args.Get(2).(*structs.IndexedCARoots)
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	result, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.DCSpecificRequest{Datacenter: "dc1"})
	require.Nil(err)
	require.Equal(cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestConnectCARoot_badReqType(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &ConnectCARoot{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.NotNil(err)
	require.Contains(err.Error(), "wrong type")

}

func TestConnectCARoot_IntegrationWithCache_NotModifiedResponse(t *testing.T) {
	rpc := &MockRPC{}
	typ := &ConnectCARoot{RPC: rpc}

	id := "active-root-id"
	rpc.On("RPC", "ConnectCA.Roots", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DCSpecificRequest)
			require.True(t, req.AllowStale)
			require.True(t, req.AllowNotModifiedResponse)

			reply := args.Get(2).(*structs.IndexedCARoots)
			reply.QueryMeta.Index = 44
			reply.NotModified = true
		})

	c := cache.New(cache.Options{})
	c.RegisterType(ConnectCARootName, typ)
	last := cache.FetchResult{
		Value: &structs.IndexedCARoots{
			ActiveRootID: id,
			QueryMeta:    structs.QueryMeta{Index: 42},
		},
		Index: 42,
	}
	req := &structs.DCSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{
			Token:         "token",
			MinQueryIndex: 44,
			MaxQueryTime:  time.Second,
		},
	}

	err := c.Prepopulate(ConnectCARootName, last, "dc1", "token", req.CacheInfo().Key)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	actual, _, err := c.Get(ctx, ConnectCARootName, req)
	require.NoError(t, err)

	expected := &structs.IndexedCARoots{
		ActiveRootID: id,
		QueryMeta:    structs.QueryMeta{Index: 42},
	}
	require.Equal(t, expected, actual)

	rpc.AssertExpectations(t)
}
