// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/rpcclient"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
)

func TestClient_SamenessGroup_BackendRouting(t *testing.T) {
	type testCase struct {
		name                string
		req                 structs.ConfigEntryQuery
		useStreamingBackend bool
		expected            func(t *testing.T, c *Client, err error)
	}

	run := func(t *testing.T, tc testCase) {
		c := &Client{
			Client: rpcclient.Client{
				NetRPC:              &fakeNetRPC{},
				Cache:               &fakeCache{configEntryType: structs.SamenessGroup, configEntryName: "sg1"},
				ViewStore:           &fakeViewStore{configEntryType: structs.SamenessGroup, configEntryName: "sg1"},
				CacheName:           "cache-no-streaming",
				UseStreamingBackend: tc.useStreamingBackend,
				QueryOptionDefaults: config.ApplyDefaultQueryOptions(&config.RuntimeConfig{}),
			},
		}
		_, _, err := c.GetSamenessGroup(context.Background(), &tc.req)
		tc.expected(t, c, err)
	}

	var testCases = []testCase{
		{
			name: "rpc by default",
			req: structs.ConfigEntryQuery{
				Kind:       structs.SamenessGroup,
				Name:       "sg1",
				Datacenter: "dc1",
			},
			useStreamingBackend: true,
			expected:            useRPC,
		},
		{
			name: "use streaming instead of cache",
			req: structs.ConfigEntryQuery{
				Kind:         structs.SamenessGroup,
				Name:         "sg1",
				QueryOptions: structs.QueryOptions{UseCache: true},
				Datacenter:   "dc1",
			},
			useStreamingBackend: true,
			expected:            useStreaming,
		},
		{
			name: "use streaming for MinQueryIndex",
			req: structs.ConfigEntryQuery{
				Kind:         structs.SamenessGroup,
				Name:         "sg1",
				Datacenter:   "dc1",
				QueryOptions: structs.QueryOptions{MinQueryIndex: 22},
			},
			useStreamingBackend: true,
			expected:            useStreaming,
		},
		{
			name: "use cache",
			req: structs.ConfigEntryQuery{
				Kind:         structs.SamenessGroup,
				Name:         "sg1",
				Datacenter:   "dc1",
				QueryOptions: structs.QueryOptions{UseCache: true},
			},
			useStreamingBackend: false,
			expected:            useCache,
		},
		{
			name: "wrong kind error",
			req: structs.ConfigEntryQuery{
				Kind: structs.ServiceDefaults,
			},
			expected: expectError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestClient_SamenessGroup_SetsDefaults(t *testing.T) {
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

	req := structs.ConfigEntryQuery{
		Datacenter:   "dc1",
		Kind:         structs.SamenessGroup,
		QueryOptions: structs.QueryOptions{MinQueryIndex: 22},
	}

	_, _, err := c.GetConfigEntry(context.Background(), &req)
	require.NoError(t, err)

	require.Len(t, store.calls, 1)
	require.Equal(t, 100*time.Second, store.calls[0].CacheInfo().Timeout)
}

func useRPC(t *testing.T, c *Client, err error) {
	t.Helper()
	require.NoError(t, err)

	rpc, ok := c.NetRPC.(*fakeNetRPC)
	require.True(t, ok, "test setup error, expected *fakeNetRPC, got %T", c.NetRPC)

	cache, ok := c.Cache.(*fakeCache)
	require.True(t, ok, "test setup error, expected *fakeCache, got %T", c.Cache)

	store, ok := c.ViewStore.(*fakeViewStore)
	require.True(t, ok, "test setup error, expected *fakeViewSTore, got %T", c.ViewStore)

	require.Len(t, cache.calls, 0)
	require.Len(t, store.calls, 0)
	require.Equal(t, []string{"ConfigEntry.Get"}, rpc.calls)
}

func useStreaming(t *testing.T, c *Client, err error) {
	t.Helper()

	require.NoError(t, err)

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

func useCache(t *testing.T, c *Client, err error) {
	t.Helper()

	require.NoError(t, err)

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

func expectError(t *testing.T, _ *Client, err error) {
	t.Helper()
	require.Error(t, err)
}

var _ rpcclient.CacheGetter = (*fakeCache)(nil)

type fakeCache struct {
	calls           []string
	configEntryType string
	configEntryName string
}

func (f *fakeCache) Get(_ context.Context, t string, _ cache.Request) (interface{}, cache.ResultMeta, error) {
	f.calls = append(f.calls, t)
	result := &structs.ConfigEntryResponse{}
	switch f.configEntryType {
	case structs.SamenessGroup:
		result = &structs.ConfigEntryResponse{
			Entry: &structs.SamenessGroupConfigEntry{
				Name: f.configEntryName,
			},
		}
	}
	return result, cache.ResultMeta{}, nil
}

func (f *fakeCache) NotifyCallback(_ context.Context, t string, _ cache.Request, _ string, _ cache.Callback) error {
	f.calls = append(f.calls, t)
	return nil
}

var _ rpcclient.NetRPC = (*fakeNetRPC)(nil)

type fakeNetRPC struct {
	calls []string
}

func (f *fakeNetRPC) RPC(ctx context.Context, method string, req interface{}, out interface{}) error {
	f.calls = append(f.calls, method)
	r, ok := req.(*structs.ConfigEntryQuery)
	if !ok {
		return errors.New("not a config entry query")
	}
	switch r.Kind {
	case structs.SamenessGroup:
		resp := &structs.ConfigEntryResponse{
			Entry: &structs.SamenessGroupConfigEntry{
				Name: r.Name,
			},
		}
		*out.(*structs.ConfigEntryResponse) = *resp
	}
	return nil
}

var _ rpcclient.MaterializedViewStore = (*fakeViewStore)(nil)

type fakeViewStore struct {
	calls           []submatview.Request
	configEntryType string
	configEntryName string
}

func (f *fakeViewStore) Get(_ context.Context, req submatview.Request) (submatview.Result, error) {
	f.calls = append(f.calls, req)
	switch f.configEntryType {
	case structs.SamenessGroup:
		return submatview.Result{Value: &structs.ConfigEntryResponse{
			Entry: &structs.SamenessGroupConfigEntry{
				Name: f.configEntryName,
			},
		}}, nil
	default:
		return submatview.Result{Value: &structs.ConfigEntryResponse{}}, nil
	}
}

func (f *fakeViewStore) NotifyCallback(_ context.Context, req submatview.Request, _ string, _ cache.Callback) error {
	f.calls = append(f.calls, req)
	return nil
}
