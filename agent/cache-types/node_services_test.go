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

func TestNodeServices(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &NodeServices{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedNodeServices
	rpc.On("RPC", "Catalog.NodeServices", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.NodeSpecificRequest)
			require.Equal(uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(1*time.Second, req.QueryOptions.MaxQueryTime)
			require.Equal("node-01", req.Node)
			require.True(req.AllowStale)

			reply := args.Get(2).(*structs.IndexedNodeServices)
			reply.NodeServices = &structs.NodeServices{
				Node: &structs.Node{
					ID:         "abcdef",
					Node:       "node-01",
					Address:    "127.0.0.5",
					Datacenter: "dc1",
				},
			}

			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "node-01",
	})
	require.NoError(err)
	require.Equal(cache.FetchResult{
		Value: resp,
		Index: 48,
	}, resultA)
}

func TestNodeServices_badReqType(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &NodeServices{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(err)
	require.Contains(err.Error(), "wrong type")

}

func TestNodeServices_IntegrationWithCache_NotModifiedResponse(t *testing.T) {
	rpc := &MockRPC{}
	typ := &NodeServices{RPC: rpc}

	services := &structs.NodeServices{
		Node: &structs.Node{
			ID:         "abcdef",
			Node:       "node-01",
			Address:    "127.0.0.5",
			Datacenter: "dc1",
		},
	}
	rpc.On("RPC", "Catalog.NodeServices", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.NodeSpecificRequest)
			require.True(t, req.AllowStale)
			require.True(t, req.AllowNotModifiedResponse)

			reply := args.Get(2).(*structs.IndexedNodeServices)
			reply.QueryMeta.Index = 44
			reply.NotModified = true
		})

	c := cache.New(cache.Options{})
	c.RegisterType(NodeServicesName, typ)
	last := cache.FetchResult{
		Value: &structs.IndexedNodeServices{
			NodeServices: services,
			QueryMeta:    structs.QueryMeta{Index: 42},
		},
		Index: 42,
	}
	req := &structs.NodeSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{
			Token:         "token",
			MinQueryIndex: 44,
			MaxQueryTime:  time.Second,
		},
	}

	err := c.Prepopulate(NodeServicesName, last, "dc1", "token", req.CacheInfo().Key)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	actual, _, err := c.Get(ctx, NodeServicesName, req)
	require.NoError(t, err)

	expected := &structs.IndexedNodeServices{
		NodeServices: services,
		QueryMeta:    structs.QueryMeta{Index: 42},
	}
	require.Equal(t, expected, actual)

	rpc.AssertExpectations(t)
}
