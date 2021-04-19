package health

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"

	"github.com/hashicorp/consul/agent/structs"
)

func TestClient_ServiceNodes_BackendRouting(t *testing.T) {
	type testCase struct {
		name     string
		req      structs.ServiceSpecificRequest
		expected func(t *testing.T, c *Client)
	}

	run := func(t *testing.T, tc testCase) {
		c := &Client{
			NetRPC:                &fakeNetRPC{},
			Cache:                 &fakeCache{},
			CacheName:             "cache-with-streaming",
			CacheNameNotStreaming: "cache-no-streaming",
		}

		_, _, err := c.ServiceNodes(context.Background(), tc.req)
		require.NoError(t, err)
		tc.expected(t, c)
	}

	var testCases = []testCase{
		{
			name: "rpc by default",
			req: structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "web1",
			},
			expected: useRPC,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func useRPC(t *testing.T, c *Client) {
	t.Helper()

	rpc, ok := c.NetRPC.(*fakeNetRPC)
	if !ok {
		t.Fatalf("test setup error, expected *fakeNetRPC, got %T", c.NetRPC)
	}

	cache, ok := c.Cache.(*fakeCache)
	if !ok {
		t.Fatalf("test setup error, expected *fakeCache, got %T", c.Cache)
	}

	require.Len(t, cache.callsGet, 0)
	require.Equal(t, []string{"Health.ServiceNodes"}, rpc.calls)
}

type fakeCache struct {
	callsGet []string
}

func (f *fakeCache) Get(_ context.Context, t string, _ cache.Request) (interface{}, cache.ResultMeta, error) {
	f.callsGet = append(f.callsGet, t)
	return nil, cache.ResultMeta{}, nil
}

func (f *fakeCache) Notify(_ context.Context, _ string, _ cache.Request, _ string, _ chan<- cache.UpdateEvent) error {
	panic("implement me")
}

type fakeNetRPC struct {
	calls []string
}

func (f *fakeNetRPC) RPC(method string, _ interface{}, _ interface{}) error {
	f.calls = append(f.calls, method)
	return nil
}

// TODO: test Notify routing
