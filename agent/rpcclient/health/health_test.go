// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/rpcclient"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
)

func TestClient_ServiceNodes_BackendRouting(t *testing.T) {
	type testCase struct {
		name     string
		req      structs.ServiceSpecificRequest
		expected func(t *testing.T, c *Client)
	}

	run := func(t *testing.T, tc testCase) {
		c := &Client{
			Client: rpcclient.Client{
				NetRPC:              &fakeNetRPC{},
				Cache:               &fakeCache{},
				ViewStore:           &fakeViewStore{},
				CacheName:           "cache-no-streaming",
				UseStreamingBackend: true,
				QueryOptionDefaults: config.ApplyDefaultQueryOptions(&config.RuntimeConfig{}),
			},
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
		{
			name: "use streaming instead of cache",
			req: structs.ServiceSpecificRequest{
				Datacenter:   "dc1",
				ServiceName:  "web1",
				QueryOptions: structs.QueryOptions{UseCache: true},
			},
			expected: useStreaming,
		},
		{
			name: "use streaming for MinQueryIndex",
			req: structs.ServiceSpecificRequest{
				Datacenter:   "dc1",
				ServiceName:  "web1",
				QueryOptions: structs.QueryOptions{MinQueryIndex: 22},
			},
			expected: useStreaming,
		},
		{
			name: "use cache for ingress request",
			req: structs.ServiceSpecificRequest{
				Datacenter:   "dc1",
				ServiceName:  "web1",
				QueryOptions: structs.QueryOptions{UseCache: true},
				Ingress:      true,
			},
			expected: useCache,
		},
		{
			name: "use cache for near request",
			req: structs.ServiceSpecificRequest{
				Datacenter:   "dc1",
				ServiceName:  "web1",
				QueryOptions: structs.QueryOptions{UseCache: true},
				Source:       structs.QuerySource{Node: "node1"},
			},
			expected: useCache,
		},
		{
			name: "rpc if merge-central-config",
			req: structs.ServiceSpecificRequest{
				Datacenter:         "dc1",
				ServiceName:        "web1",
				MergeCentralConfig: true,
				QueryOptions:       structs.QueryOptions{MinQueryIndex: 22},
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
	require.True(t, ok, "test setup error, expected *fakeNetRPC, got %T", c.NetRPC)

	cache, ok := c.Cache.(*fakeCache)
	require.True(t, ok, "test setup error, expected *fakeCache, got %T", c.Cache)

	store, ok := c.ViewStore.(*fakeViewStore)
	require.True(t, ok, "test setup error, expected *fakeViewSTore, got %T", c.ViewStore)

	require.Len(t, cache.calls, 0)
	require.Len(t, store.calls, 0)
	require.Equal(t, []string{"Health.ServiceNodes"}, rpc.calls)
}

func useStreaming(t *testing.T, c *Client) {
	t.Helper()

	rpc, ok := c.NetRPC.(*fakeNetRPC)
	require.True(t, ok, "test setup error, expected *fakeNetRPC, got %T", c.NetRPC)

	cache, ok := c.Cache.(*fakeCache)
	require.True(t, ok, "test setup error, expected *fakeCache, got %T", c.Cache)

	store, ok := c.ViewStore.(*fakeViewStore)
	require.True(t, ok, "test setup error, expected *fakeViewSTore, got %T", c.ViewStore)

	require.Len(t, cache.calls, 0)
	require.Len(t, rpc.calls, 0)
	require.Len(t, store.calls, 1)
}

func useCache(t *testing.T, c *Client) {
	t.Helper()

	rpc, ok := c.NetRPC.(*fakeNetRPC)
	require.True(t, ok, "test setup error, expected *fakeNetRPC, got %T", c.NetRPC)

	cache, ok := c.Cache.(*fakeCache)
	require.True(t, ok, "test setup error, expected *fakeCache, got %T", c.Cache)

	store, ok := c.ViewStore.(*fakeViewStore)
	require.True(t, ok, "test setup error, expected *fakeViewSTore, got %T", c.ViewStore)

	require.Len(t, rpc.calls, 0)
	require.Len(t, store.calls, 0)
	require.Equal(t, []string{"cache-no-streaming"}, cache.calls)
}

type fakeCache struct {
	calls []string
}

func (f *fakeCache) Get(_ context.Context, t string, _ cache.Request) (interface{}, cache.ResultMeta, error) {
	f.calls = append(f.calls, t)
	result := &structs.IndexedCheckServiceNodes{}
	return result, cache.ResultMeta{}, nil
}

func (f *fakeCache) NotifyCallback(_ context.Context, t string, _ cache.Request, _ string, _ cache.Callback) error {
	f.calls = append(f.calls, t)
	return nil
}

type fakeNetRPC struct {
	calls []string
}

func (f *fakeNetRPC) RPC(ctx context.Context, method string, _ interface{}, _ interface{}) error {
	f.calls = append(f.calls, method)
	return nil
}

type fakeViewStore struct {
	calls []submatview.Request
}

func (f *fakeViewStore) Get(_ context.Context, req submatview.Request) (submatview.Result, error) {
	f.calls = append(f.calls, req)
	return submatview.Result{Value: &structs.IndexedCheckServiceNodes{}}, nil
}

func (f *fakeViewStore) NotifyCallback(_ context.Context, req submatview.Request, _ string, _ cache.Callback) error {
	f.calls = append(f.calls, req)
	return nil
}

func TestClient_Notify_BackendRouting(t *testing.T) {
	type testCase struct {
		name     string
		req      structs.ServiceSpecificRequest
		expected func(t *testing.T, c *Client)
	}

	run := func(t *testing.T, tc testCase) {
		c := &Client{
			Client: rpcclient.Client{
				NetRPC:              &fakeNetRPC{},
				Cache:               &fakeCache{},
				ViewStore:           &fakeViewStore{},
				CacheName:           "cache-no-streaming",
				UseStreamingBackend: true,
			},
		}

		err := c.Notify(context.Background(), tc.req, "cid", nil)
		require.NoError(t, err)
		tc.expected(t, c)
	}

	var testCases = []testCase{
		{
			name: "streaming by default",
			req: structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "web1",
			},
			expected: useStreaming,
		},
		{
			name: "use cache for ingress request",
			req: structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "web1",
				Ingress:     true,
			},
			expected: useCache,
		},
		{
			name: "use cache for near request",
			req: structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "web1",
				Source:      structs.QuerySource{Node: "node1"},
			},
			expected: useCache,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestClient_ServiceNodes_SetsDefaults(t *testing.T) {
	store := &fakeViewStore{}
	c := &Client{
		Client: rpcclient.Client{
			ViewStore:           store,
			CacheName:           "cache-no-streaming",
			UseStreamingBackend: true,
			QueryOptionDefaults: config.ApplyDefaultQueryOptions(&config.RuntimeConfig{
				MaxQueryTime:     200 * time.Second,
				DefaultQueryTime: 100 * time.Second,
			}),
		},
	}

	req := structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "web1",
		QueryOptions: structs.QueryOptions{MinQueryIndex: 22},
	}

	_, _, err := c.ServiceNodes(context.Background(), req)
	require.NoError(t, err)

	require.Len(t, store.calls, 1)
	require.Equal(t, 100*time.Second, store.calls[0].CacheInfo().Timeout)
}
