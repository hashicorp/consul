package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntry_DiscoveryChain(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	config_entries := c.ConfigEntries()

	verifyResolver := func(t *testing.T, initial ConfigEntry) {
		t.Helper()
		require.IsType(t, &ServiceResolverConfigEntry{}, initial)
		testEntry := initial.(*ServiceResolverConfigEntry)

		// set it
		_, wm, err := config_entries.Set(testEntry, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config_entries.Get(ServiceResolver, testEntry.Name, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readResolver, ok := entry.(*ServiceResolverConfigEntry)
		require.True(t, ok)
		readResolver.ModifyIndex = 0 // reset for Equals()
		readResolver.CreateIndex = 0 // reset for Equals()

		require.Equal(t, testEntry, readResolver)

		// TODO(rb): cas?
		// TODO(rb): list?
	}

	verifySplitter := func(t *testing.T, initial ConfigEntry) {
		t.Helper()
		require.IsType(t, &ServiceSplitterConfigEntry{}, initial)
		testEntry := initial.(*ServiceSplitterConfigEntry)

		// set it
		_, wm, err := config_entries.Set(testEntry, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config_entries.Get(ServiceSplitter, testEntry.Name, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readSplitter, ok := entry.(*ServiceSplitterConfigEntry)
		require.True(t, ok)
		readSplitter.ModifyIndex = 0 // reset for Equals()
		readSplitter.CreateIndex = 0 // reset for Equals()

		require.Equal(t, testEntry, readSplitter)

		// TODO(rb): cas?
		// TODO(rb): list?
	}

	verifyRouter := func(t *testing.T, initial ConfigEntry) {
		t.Helper()
		require.IsType(t, &ServiceRouterConfigEntry{}, initial)
		testEntry := initial.(*ServiceRouterConfigEntry)

		// set it
		_, wm, err := config_entries.Set(testEntry, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config_entries.Get(ServiceRouter, testEntry.Name, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readRouter, ok := entry.(*ServiceRouterConfigEntry)
		require.True(t, ok)
		readRouter.ModifyIndex = 0 // reset for Equals()
		readRouter.CreateIndex = 0 // reset for Equals()

		require.Equal(t, testEntry, readRouter)

		// TODO(rb): cas?
		// TODO(rb): list?
	}

	// First set the necessary protocols to allow advanced routing features.
	for _, service := range []string{
		"test-failover",
		"test-redirect",
		"alternate",
		"test-split",
		"test-route",
	} {
		serviceDefaults := &ServiceConfigEntry{
			Kind:     ServiceDefaults,
			Name:     service,
			Protocol: "http",
		}
		_, _, err := config_entries.Set(serviceDefaults, nil)
		require.NoError(t, err)
	}

	// NOTE: Due to service graph validation, these have to happen in a specific order.
	for _, tc := range []struct {
		name   string
		entry  ConfigEntry
		verify func(t *testing.T, initial ConfigEntry)
	}{
		{
			name: "failover",
			entry: &ServiceResolverConfigEntry{
				Kind:          ServiceResolver,
				Name:          "test-failover",
				DefaultSubset: "v1",
				Subsets: map[string]ServiceResolverSubset{
					"v1": ServiceResolverSubset{
						Filter: "Service.Meta.version == v1",
					},
					"v2": ServiceResolverSubset{
						Filter: "Service.Meta.version == v2",
					},
				},
				Failover: map[string]ServiceResolverFailover{
					"*": ServiceResolverFailover{
						Datacenters: []string{"dc2"},
					},
					"v1": ServiceResolverFailover{
						Service: "alternate",
					},
				},
				ConnectTimeout: 5 * time.Second,
			},
			verify: verifyResolver,
		},
		{
			name: "redirect",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test-redirect",
				Redirect: &ServiceResolverRedirect{
					Service:       "test-failover",
					ServiceSubset: "v2",
					Namespace:     "c",
					Datacenter:    "d",
				},
			},
			verify: verifyResolver,
		},
		{
			name: "mega splitter", // use one mega object to avoid multiple trips
			entry: &ServiceSplitterConfigEntry{
				Kind: ServiceSplitter,
				Name: "test-split",
				Splits: []ServiceSplit{
					{
						Weight:        90,
						Service:       "test-failover",
						ServiceSubset: "v1",
						Namespace:     "c",
					},
					{
						Weight:    10,
						Service:   "test-redirect",
						Namespace: "z",
					},
				},
			},
			verify: verifySplitter,
		},
		{
			name: "mega router", // use one mega object to avoid multiple trips
			entry: &ServiceRouterConfigEntry{
				Kind: ServiceRouter,
				Name: "test-route",
				Routes: []ServiceRoute{
					{
						Match: &ServiceRouteMatch{
							HTTP: &ServiceRouteHTTPMatch{
								PathPrefix: "/prefix",
								Header: []ServiceRouteHTTPMatchHeader{
									{Name: "x-debug", Exact: "1"},
								},
								QueryParam: []ServiceRouteHTTPMatchQueryParam{
									{Name: "debug", Exact: "1"},
								},
							},
						},
						Destination: &ServiceRouteDestination{
							Service:               "test-failover",
							ServiceSubset:         "v2",
							Namespace:             "sec",
							PrefixRewrite:         "/",
							RequestTimeout:        5 * time.Second,
							NumRetries:            5,
							RetryOnConnectFailure: true,
							RetryOnStatusCodes:    []uint32{500, 503, 401},
						},
					},
				},
			},
			verify: verifyRouter,
		},
	} {
		tc := tc
		name := fmt.Sprintf("%s:%s: %s", tc.entry.GetKind(), tc.entry.GetName(), tc.name)
		ok := t.Run(name, func(t *testing.T) {
			tc.verify(t, tc.entry)
		})
		require.True(t, ok, "subtest %q failed so aborting remainder", name)
	}
}
