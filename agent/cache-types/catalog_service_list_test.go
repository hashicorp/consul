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

func TestCatalogServiceList(t *testing.T) {
	rpc := TestRPC(t)
	typ := &CatalogServiceList{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedServiceList
	rpc.On("RPC", "Catalog.ServiceList", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DCSpecificRequest)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)

			reply := args.Get(2).(*structs.IndexedServiceList)
			reply.Services = structs.ServiceList{
				structs.ServiceName{
					Name: "foo",
				},
				structs.ServiceName{
					Name: "bar",
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

func TestCatalogServiceList_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	typ := &CatalogServiceList{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
	rpc.AssertExpectations(t)
}

func TestCatalogServiceList_IntegrationWithCache_NotModifiedResponse(t *testing.T) {
	rpc := &MockRPC{}
	typ := &CatalogServiceList{RPC: rpc}

	services := structs.ServiceList{
		{Name: "service"},
	}
	rpc.On("RPC", "Catalog.ServiceList", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DCSpecificRequest)
			require.True(t, req.AllowStale)
			require.True(t, req.AllowNotModifiedResponse)

			reply := args.Get(2).(*structs.IndexedServiceList)
			reply.QueryMeta.Index = 44
			reply.NotModified = true
		})

	c := cache.New(cache.Options{})
	c.RegisterType(CatalogServiceListName, typ)
	last := cache.FetchResult{
		Value: &structs.IndexedServiceList{
			Services:  services,
			QueryMeta: structs.QueryMeta{Index: 42},
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

	err := c.Prepopulate(CatalogServiceListName, last, "dc1", "token", req.CacheInfo().Key)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	actual, _, err := c.Get(ctx, CatalogServiceListName, req)
	require.NoError(t, err)

	expected := &structs.IndexedServiceList{
		Services:  services,
		QueryMeta: structs.QueryMeta{Index: 42},
	}
	require.Equal(t, expected, actual)

	rpc.AssertExpectations(t)
}
