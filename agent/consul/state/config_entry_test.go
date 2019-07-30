package state

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
)

func TestStore_ConfigEntry(t *testing.T) {
	require := require.New(t)
	s := testStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Create
	require.NoError(s.EnsureConfigEntry(0, expected))

	idx, config, err := s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(0), idx)
	require.Equal(expected, config)

	// Update
	updated := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "bar",
		},
	}
	require.NoError(s.EnsureConfigEntry(1, updated))

	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(1), idx)
	require.Equal(updated, config)

	// Delete
	require.NoError(s.DeleteConfigEntry(2, structs.ProxyDefaults, "global"))

	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Nil(config)

	// Set up a watch.
	serviceConf := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}
	require.NoError(s.EnsureConfigEntry(3, serviceConf))

	ws := memdb.NewWatchSet()
	_, _, err = s.ConfigEntry(ws, structs.ServiceDefaults, "foo")
	require.NoError(err)

	// Make an unrelated modification and make sure the watch doesn't fire.
	require.NoError(s.EnsureConfigEntry(4, updated))
	require.False(watchFired(ws))

	// Update the watched config and make sure it fires.
	serviceConf.Protocol = "http"
	require.NoError(s.EnsureConfigEntry(5, serviceConf))
	require.True(watchFired(ws))
}

func TestStore_ConfigEntryCAS(t *testing.T) {
	require := require.New(t)
	s := testStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Create
	require.NoError(s.EnsureConfigEntry(1, expected))

	idx, config, err := s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(1), idx)
	require.Equal(expected, config)

	// Update with invalid index
	updated := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "bar",
		},
	}
	ok, err := s.EnsureConfigEntryCAS(2, 99, updated)
	require.False(ok)
	require.NoError(err)

	// Entry should not be changed
	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(1), idx)
	require.Equal(expected, config)

	// Update with a valid index
	ok, err = s.EnsureConfigEntryCAS(2, 1, updated)
	require.True(ok)
	require.NoError(err)

	// Entry should be updated
	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal(updated, config)
}

func TestStore_ConfigEntries(t *testing.T) {
	require := require.New(t)
	s := testStateStore(t)

	// Create some config entries.
	entry1 := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "test1",
	}
	entry2 := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "test2",
	}
	entry3 := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "test3",
	}

	require.NoError(s.EnsureConfigEntry(0, entry1))
	require.NoError(s.EnsureConfigEntry(1, entry2))
	require.NoError(s.EnsureConfigEntry(2, entry3))

	// Get all entries
	idx, entries, err := s.ConfigEntries(nil)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal([]structs.ConfigEntry{entry1, entry2, entry3}, entries)

	// Get all proxy entries
	idx, entries, err = s.ConfigEntriesByKind(nil, structs.ProxyDefaults)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal([]structs.ConfigEntry{entry1}, entries)

	// Get all service entries
	ws := memdb.NewWatchSet()
	idx, entries, err = s.ConfigEntriesByKind(ws, structs.ServiceDefaults)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal([]structs.ConfigEntry{entry2, entry3}, entries)

	// Watch should not have fired
	require.False(watchFired(ws))

	// Now make an update and make sure the watch fires.
	require.NoError(s.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "test2",
		Protocol: "tcp",
	}))
	require.True(watchFired(ws))
}

func TestStore_ConfigEntry_GraphValidation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		entries        []structs.ConfigEntry
		op             func(t *testing.T, s *Store) error
		expectErr      string
		expectGraphErr bool
	}{
		{
			name:    "splitter fails without default protocol",
			entries: []structs.ConfigEntry{},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Namespace: "v1"},
						{Weight: 10, Namespace: "v2"},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		{
			name: "splitter fails with tcp protocol",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Namespace: "v1"},
						{Weight: 10, Namespace: "v2"},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		{
			name: "splitter works with http protocol",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "tcp", // loses
					},
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Namespace: "v1"},
						{Weight: 10, Namespace: "v2"},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
		},
		{
			name: "splitter works with http protocol (from proxy-defaults)",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Namespace: "v1"},
						{Weight: 10, Namespace: "v2"},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
		},
		{
			name: "router fails with tcp protocol",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/other",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Namespace: "other",
							},
						},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		{
			name:    "router fails without default protocol",
			entries: []structs.ConfigEntry{},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/other",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Namespace: "other",
							},
						},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		{
			name: "cannot remove default protocol after splitter created",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Namespace: "v1"},
						{Weight: 10, Namespace: "v2"},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				return s.DeleteConfigEntry(0, structs.ServiceDefaults, "main")
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		{
			name: "cannot remove global default protocol after splitter created",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Namespace: "v1"},
						{Weight: 10, Namespace: "v2"},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				return s.DeleteConfigEntry(0, structs.ProxyDefaults, structs.ProxyConfigGlobal)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		{
			name: "can remove global default protocol after splitter created if service default overrides it",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Namespace: "v1"},
						{Weight: 10, Namespace: "v2"},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				return s.DeleteConfigEntry(0, structs.ProxyDefaults, structs.ProxyConfigGlobal)
			},
		},
		{
			name: "cannot change to tcp protocol after splitter created",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Namespace: "v1"},
						{Weight: 10, Namespace: "v2"},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		{
			name: "cannot remove default protocol after router created",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/other",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Namespace: "other",
							},
						},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				return s.DeleteConfigEntry(0, structs.ServiceDefaults, "main")
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		{
			name: "cannot change to tcp protocol after router created",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/other",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Namespace: "other",
							},
						},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		{
			name: "cannot split to a service using tcp",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "tcp",
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90},
						{Weight: 10, Service: "other"},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		{
			name: "cannot route to a service using tcp",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "tcp",
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/other",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "other",
							},
						},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		{
			name: "cannot failover to a service using a different protocol",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "grpc",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "tcp",
				},
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "main",
					ConnectTimeout: 33 * time.Second,
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": structs.ServiceResolverFailover{
							Service: "other",
						},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		{
			name: "cannot redirect to a service using a different protocol",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "grpc",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "tcp",
				},
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "main",
					ConnectTimeout: 33 * time.Second,
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "other",
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		{
			name: "redirect to a subset that does exist is fine",
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "other",
					ConnectTimeout: 33 * time.Second,
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": structs.ServiceResolverSubset{
							Filter: "Service.Meta.version == v1",
						},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Redirect: &structs.ServiceResolverRedirect{
						Service:       "other",
						ServiceSubset: "v1",
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
		},
		{
			name: "cannot redirect to a subset that does not exist",
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "other",
					ConnectTimeout: 33 * time.Second,
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Redirect: &structs.ServiceResolverRedirect{
						Service:       "other",
						ServiceSubset: "v1",
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      `does not have a subset named "v1"`,
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		{
			name: "cannot introduce circular resolver redirect",
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "other",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "main",
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "other",
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      `detected circular resolver redirect`,
			expectGraphErr: true,
		},
		{
			name: "cannot introduce circular split",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: "service-splitter",
					Name: "other",
					Splits: []structs.ServiceSplit{
						{Weight: 100, Service: "main"},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: "service-splitter",
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 100, Service: "other"},
					},
				}
				return s.EnsureConfigEntry(0, entry)
			},
			expectErr:      `detected circular reference`,
			expectGraphErr: true,
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			for _, entry := range tc.entries {
				require.NoError(t, s.EnsureConfigEntry(0, entry))
			}

			err := tc.op(t, s)
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
				_, ok := err.(*structs.ConfigEntryGraphError)
				if tc.expectGraphErr {
					require.True(t, ok, "%T is not a *ConfigEntryGraphError", err)
				} else {
					require.False(t, ok, "did not expect a *ConfigEntryGraphError here: %v", err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStore_ReadDiscoveryChainConfigEntries_Overrides(t *testing.T) {
	for _, tc := range []struct {
		name           string
		entries        []structs.ConfigEntry
		expectBefore   []structs.ConfigEntryKindName
		overrides      map[structs.ConfigEntryKindName]structs.ConfigEntry
		expectAfter    []structs.ConfigEntryKindName
		expectAfterErr string
		checkAfter     func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries)
	}{
		{
			name: "mask service-defaults",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
			},
			expectBefore: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				{Kind: structs.ServiceDefaults, Name: "main"}: nil,
			},
			expectAfter: []structs.ConfigEntryKindName{
				// nothing
			},
		},
		{
			name: "edit service-defaults",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
			},
			expectBefore: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				{Kind: structs.ServiceDefaults, Name: "main"}: &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "grpc",
				},
			},
			expectAfter: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
			},
			checkAfter: func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries) {
				defaults := entrySet.GetService("main")
				require.NotNil(t, defaults)
				require.Equal(t, "grpc", defaults.Protocol)
			},
		},

		{
			name: "mask service-router",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
				},
			},
			expectBefore: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
				{Kind: structs.ServiceRouter, Name: "main"},
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				{Kind: structs.ServiceRouter, Name: "main"}: nil,
			},
			expectAfter: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
			},
		},
		{
			name: "edit service-router",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {Filter: "Service.Meta.version == v1"},
						"v2": {Filter: "Service.Meta.version == v2"},
						"v3": {Filter: "Service.Meta.version == v3"},
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/admin",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								ServiceSubset: "v2",
							},
						},
					},
				},
			},
			expectBefore: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
				{Kind: structs.ServiceResolver, Name: "main"},
				{Kind: structs.ServiceRouter, Name: "main"},
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				{Kind: structs.ServiceRouter, Name: "main"}: &structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/admin",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								ServiceSubset: "v3",
							},
						},
					},
				},
			},
			expectAfter: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
				{Kind: structs.ServiceResolver, Name: "main"},
				{Kind: structs.ServiceRouter, Name: "main"},
			},
			checkAfter: func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries) {
				router := entrySet.GetRouter("main")
				require.NotNil(t, router)
				require.Len(t, router.Routes, 1)

				expect := structs.ServiceRoute{
					Match: &structs.ServiceRouteMatch{
						HTTP: &structs.ServiceRouteHTTPMatch{
							PathExact: "/admin",
						},
					},
					Destination: &structs.ServiceRouteDestination{
						ServiceSubset: "v3",
					},
				}
				require.Equal(t, expect, router.Routes[0])
			},
		},

		{
			name: "mask service-splitter",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 100},
					},
				},
			},
			expectBefore: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
				{Kind: structs.ServiceSplitter, Name: "main"},
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				{Kind: structs.ServiceSplitter, Name: "main"}: nil,
			},
			expectAfter: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
			},
		},
		{
			name: "edit service-splitter",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 100},
					},
				},
			},
			expectBefore: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
				{Kind: structs.ServiceSplitter, Name: "main"},
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				{Kind: structs.ServiceSplitter, Name: "main"}: &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 85, ServiceSubset: "v1"},
						{Weight: 15, ServiceSubset: "v2"},
					},
				},
			},
			expectAfter: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceDefaults, Name: "main"},
				{Kind: structs.ServiceSplitter, Name: "main"},
			},
			checkAfter: func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries) {
				splitter := entrySet.GetSplitter("main")
				require.NotNil(t, splitter)
				require.Len(t, splitter.Splits, 2)

				expect := []structs.ServiceSplit{
					{Weight: 85, ServiceSubset: "v1"},
					{Weight: 15, ServiceSubset: "v2"},
				}
				require.Equal(t, expect, splitter.Splits)
			},
		},

		{
			name: "mask service-resolver",
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
				},
			},
			expectBefore: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceResolver, Name: "main"},
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				{Kind: structs.ServiceResolver, Name: "main"}: nil,
			},
			expectAfter: []structs.ConfigEntryKindName{
				// nothing
			},
		},
		{
			name: "edit service-resolver",
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
				},
			},
			expectBefore: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceResolver, Name: "main"},
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				{Kind: structs.ServiceResolver, Name: "main"}: &structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "main",
					ConnectTimeout: 33 * time.Second,
				},
			},
			expectAfter: []structs.ConfigEntryKindName{
				{Kind: structs.ServiceResolver, Name: "main"},
			},
			checkAfter: func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries) {
				resolver := entrySet.GetResolver("main")
				require.NotNil(t, resolver)
				require.Equal(t, 33*time.Second, resolver.ConnectTimeout)
			},
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			for _, entry := range tc.entries {
				require.NoError(t, s.EnsureConfigEntry(0, entry))
			}

			t.Run("without override", func(t *testing.T) {
				_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", nil)
				require.NoError(t, err)
				got := entrySetToKindNames(entrySet)
				require.ElementsMatch(t, tc.expectBefore, got)
			})

			t.Run("with override", func(t *testing.T) {
				_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", tc.overrides)

				if tc.expectAfterErr != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), tc.expectAfterErr)
				} else {
					require.NoError(t, err)
					got := entrySetToKindNames(entrySet)
					require.ElementsMatch(t, tc.expectAfter, got)

					if tc.checkAfter != nil {
						tc.checkAfter(t, entrySet)
					}
				}
			})
		})
	}
}

func entrySetToKindNames(entrySet *structs.DiscoveryChainConfigEntries) []structs.ConfigEntryKindName {
	var out []structs.ConfigEntryKindName
	for _, entry := range entrySet.Routers {
		out = append(out, structs.ConfigEntryKindName{
			Kind: entry.Kind,
			Name: entry.Name,
		})
	}
	for _, entry := range entrySet.Splitters {
		out = append(out, structs.ConfigEntryKindName{
			Kind: entry.Kind,
			Name: entry.Name,
		})
	}
	for _, entry := range entrySet.Resolvers {
		out = append(out, structs.ConfigEntryKindName{
			Kind: entry.Kind,
			Name: entry.Name,
		})
	}
	for _, entry := range entrySet.Services {
		out = append(out, structs.ConfigEntryKindName{
			Kind: entry.Kind,
			Name: entry.Name,
		})
	}
	return out
}

func TestStore_ReadDiscoveryChainConfigEntries_SubsetSplit(t *testing.T) {
	s := testStateStore(t)

	entries := []structs.ConfigEntry{
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "main",
			Protocol: "http",
		},
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "main",
			Subsets: map[string]structs.ServiceResolverSubset{
				"v1": structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == v1",
				},
				"v2": structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == v2",
				},
			},
		},
		&structs.ServiceSplitterConfigEntry{
			Kind: structs.ServiceSplitter,
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 90, ServiceSubset: "v1"},
				{Weight: 10, ServiceSubset: "v2"},
			},
		},
	}

	for _, entry := range entries {
		require.NoError(t, s.EnsureConfigEntry(0, entry))
	}

	_, entrySet, err := s.ReadDiscoveryChainConfigEntries(nil, "main")
	require.NoError(t, err)

	require.Len(t, entrySet.Routers, 0)
	require.Len(t, entrySet.Splitters, 1)
	require.Len(t, entrySet.Resolvers, 1)
	require.Len(t, entrySet.Services, 1)
}
