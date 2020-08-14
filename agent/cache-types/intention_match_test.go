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

func TestIntentionMatch(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &IntentionMatch{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedIntentionMatches
	rpc.On("RPC", "Intention.Match", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.IntentionQueryRequest)
			require.Equal(uint64(24), req.MinQueryIndex)
			require.Equal(1*time.Second, req.MaxQueryTime)

			reply := args.Get(2).(*structs.IndexedIntentionMatches)
			reply.Index = 48
			resp = reply
		})

	// Fetch
	result, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.IntentionQueryRequest{Datacenter: "dc1"})
	require.NoError(err)
	require.Equal(cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestIntentionMatch_badReqType(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &IntentionMatch{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(err)
	require.Contains(err.Error(), "wrong type")

}

func TestIntentionMatch_IntegrationWithCache_NotModifiedResponse(t *testing.T) {
	rpc := &MockRPC{}
	typ := &IntentionMatch{RPC: rpc}

	intentions := []structs.Intentions{
		{&structs.Intention{ID: "id"}},
	}
	rpc.On("RPC", "Intention.Match", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.IntentionQueryRequest)
			require.True(t, req.AllowStale)
			require.True(t, req.AllowNotModifiedResponse)

			reply := args.Get(2).(*structs.IndexedIntentionMatches)
			reply.QueryMeta.Index = 44
			reply.NotModified = true
		})

	c := cache.New(cache.Options{})
	c.RegisterType(IntentionMatchName, typ)
	last := cache.FetchResult{
		Value: &structs.IndexedIntentionMatches{
			Matches:   intentions,
			QueryMeta: structs.QueryMeta{Index: 42},
		},
		Index: 42,
	}
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{
			Token:         "token",
			MinQueryIndex: 44,
			MaxQueryTime:  time.Second,
		},
		Match: &structs.IntentionQueryMatch{},
	}

	err := c.Prepopulate(IntentionMatchName, last, "dc1", "token", req.CacheInfo().Key)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	actual, _, err := c.Get(ctx, IntentionMatchName, req)
	require.NoError(t, err)

	expected := &structs.IndexedIntentionMatches{
		Matches:   intentions,
		QueryMeta: structs.QueryMeta{Index: 42},
	}
	require.Equal(t, expected, actual)

	rpc.AssertExpectations(t)
}
