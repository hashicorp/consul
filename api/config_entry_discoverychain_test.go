package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntry_DiscoveryChain(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	config_entries := c.ConfigEntries()

	t.Run("Service Router", func(t *testing.T) {
		// use one mega object to avoid multiple trips
		makeEntry := func() *ServiceRouterConfigEntry {
			return &ServiceRouterConfigEntry{
				Kind: ServiceRouter,
				Name: "test",
				Routes: []ServiceRoute{
					{
						Match: &ServiceRouteMatch{
							HTTP: &ServiceRouteHTTPMatch{
								PathPrefix: "/prefix",
								Header: []ServiceRouteHTTPMatchHeader{
									{Name: "x-debug", Exact: "1"},
								},
								QueryParam: []ServiceRouteHTTPMatchQueryParam{
									{Name: "debug", Value: "1"},
								},
								Methods: []string{"GET", "POST"},
							},
						},
						Destination: &ServiceRouteDestination{
							Service:               "other",
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
			}
		}

		// set it
		_, wm, err := config_entries.Set(makeEntry(), nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config_entries.Get(ServiceRouter, "test", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readRouter, ok := entry.(*ServiceRouterConfigEntry)
		require.True(t, ok)
		readRouter.ModifyIndex = 0 // reset for Equals()
		readRouter.CreateIndex = 0 // reset for Equals()

		goldenEntry := makeEntry()
		require.Equal(t, goldenEntry, readRouter)

		// TODO(rb): cas?
		// TODO(rb): list?
	})

	t.Run("Service Splitter", func(t *testing.T) {
		// use one mega object to avoid multiple trips
		makeEntry := func() *ServiceSplitterConfigEntry {
			return &ServiceSplitterConfigEntry{
				Kind: ServiceSplitter,
				Name: "test",
				Splits: []ServiceSplit{
					{
						Weight:        90,
						Service:       "a",
						ServiceSubset: "b",
						Namespace:     "c",
					},
					{
						Weight:        10,
						Service:       "x",
						ServiceSubset: "y",
						Namespace:     "z",
					},
				},
			}
		}

		// set it
		_, wm, err := config_entries.Set(makeEntry(), nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config_entries.Get(ServiceSplitter, "test", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readSplitter, ok := entry.(*ServiceSplitterConfigEntry)
		require.True(t, ok)
		readSplitter.ModifyIndex = 0 // reset for Equals()
		readSplitter.CreateIndex = 0 // reset for Equals()

		goldenEntry := makeEntry()
		require.Equal(t, goldenEntry, readSplitter)

		// TODO(rb): cas?
		// TODO(rb): list?
	})

	for name, tc := range map[string]func() *ServiceResolverConfigEntry{
		"with-redirect": func() *ServiceResolverConfigEntry {
			return &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service:       "a",
					ServiceSubset: "b",
					Namespace:     "c",
					Datacenter:    "d",
				},
			}
		},
		"no-redirect": func() *ServiceResolverConfigEntry {
			return &ServiceResolverConfigEntry{
				Kind:          ServiceResolver,
				Name:          "test",
				DefaultSubset: "v1",
				Subsets: map[string]ServiceResolverSubset{
					"v1": ServiceResolverSubset{
						Filter: "ServiceMeta.version == v1",
					},
					"v2": ServiceResolverSubset{
						Filter: "ServiceMeta.version == v2",
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
			}
		},
	} {
		// use one mega object to avoid multiple trips
		makeEntry := tc
		t.Run("Service Resolver - "+name, func(t *testing.T) {

			// set it
			_, wm, err := config_entries.Set(makeEntry(), nil)
			require.NoError(t, err)
			require.NotNil(t, wm)
			require.NotEqual(t, 0, wm.RequestTime)

			// get it
			entry, qm, err := config_entries.Get(ServiceResolver, "test", nil)
			require.NoError(t, err)
			require.NotNil(t, qm)
			require.NotEqual(t, 0, qm.RequestTime)

			// verify it
			readResolver, ok := entry.(*ServiceResolverConfigEntry)
			require.True(t, ok)
			readResolver.ModifyIndex = 0 // reset for Equals()
			readResolver.CreateIndex = 0 // reset for Equals()

			goldenEntry := makeEntry()
			require.Equal(t, goldenEntry, readResolver)

			// TODO(rb): cas?
			// TODO(rb): list?
		})
	}
}
