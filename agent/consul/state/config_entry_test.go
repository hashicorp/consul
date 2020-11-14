package state

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
)

func TestStore_ConfigEntry(t *testing.T) {
	require := require.New(t)
	s := testConfigStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Create
	require.NoError(s.EnsureConfigEntry(0, expected, nil))

	idx, config, err := s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
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
	require.NoError(s.EnsureConfigEntry(1, updated, nil))

	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(err)
	require.Equal(uint64(1), idx)
	require.Equal(updated, config)

	// Delete
	require.NoError(s.DeleteConfigEntry(2, structs.ProxyDefaults, "global", nil))

	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Nil(config)

	// Set up a watch.
	serviceConf := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}
	require.NoError(s.EnsureConfigEntry(3, serviceConf, nil))

	ws := memdb.NewWatchSet()
	_, _, err = s.ConfigEntry(ws, structs.ServiceDefaults, "foo", nil)
	require.NoError(err)

	// Make an unrelated modification and make sure the watch doesn't fire.
	require.NoError(s.EnsureConfigEntry(4, updated, nil))
	require.False(watchFired(ws))

	// Update the watched config and make sure it fires.
	serviceConf.Protocol = "http"
	require.NoError(s.EnsureConfigEntry(5, serviceConf, nil))
	require.True(watchFired(ws))
}

func TestStore_ConfigEntryCAS(t *testing.T) {
	require := require.New(t)
	s := testConfigStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Create
	require.NoError(s.EnsureConfigEntry(1, expected, nil))

	idx, config, err := s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
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
	ok, err := s.EnsureConfigEntryCAS(2, 99, updated, nil)
	require.False(ok)
	require.NoError(err)

	// Entry should not be changed
	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(err)
	require.Equal(uint64(1), idx)
	require.Equal(expected, config)

	// Update with a valid index
	ok, err = s.EnsureConfigEntryCAS(2, 1, updated, nil)
	require.True(ok)
	require.NoError(err)

	// Entry should be updated
	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal(updated, config)
}

func TestStore_ConfigEntry_UpdateOver(t *testing.T) {
	// This test uses ServiceIntentions because they are the only
	// kind that implements UpdateOver() at this time.

	s := testConfigStateStore(t)

	var (
		idA = testUUID()
		idB = testUUID()

		loc   = time.FixedZone("UTC-8", -8*60*60)
		timeA = time.Date(1955, 11, 5, 6, 15, 0, 0, loc)
		timeB = time.Date(1985, 10, 26, 1, 35, 0, 0, loc)
	)
	require.NotEqual(t, idA, idB)

	initial := &structs.ServiceIntentionsConfigEntry{
		Kind: structs.ServiceIntentions,
		Name: "api",
		Sources: []*structs.SourceIntention{
			{
				LegacyID:         idA,
				Name:             "web",
				Action:           structs.IntentionActionAllow,
				LegacyCreateTime: &timeA,
				LegacyUpdateTime: &timeA,
			},
		},
	}

	// Create
	nextIndex := uint64(1)
	require.NoError(t, s.EnsureConfigEntry(nextIndex, initial.Clone(), nil))

	idx, raw, err := s.ConfigEntry(nil, structs.ServiceIntentions, "api", nil)
	require.NoError(t, err)
	require.Equal(t, nextIndex, idx)

	got, ok := raw.(*structs.ServiceIntentionsConfigEntry)
	require.True(t, ok)
	initial.RaftIndex = got.RaftIndex
	require.Equal(t, initial, got)

	t.Run("update and fail change legacyID", func(t *testing.T) {
		// Update
		updated := &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "api",
			Sources: []*structs.SourceIntention{
				{
					LegacyID:         idB,
					Name:             "web",
					Action:           structs.IntentionActionDeny,
					LegacyCreateTime: &timeB,
					LegacyUpdateTime: &timeB,
				},
			},
		}

		nextIndex++
		err := s.EnsureConfigEntry(nextIndex, updated.Clone(), nil)
		testutil.RequireErrorContains(t, err, "cannot set this field to a different value")
	})

	t.Run("update and do not update create time", func(t *testing.T) {
		// Update
		updated := &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "api",
			Sources: []*structs.SourceIntention{
				{
					LegacyID:         idA,
					Name:             "web",
					Action:           structs.IntentionActionDeny,
					LegacyCreateTime: &timeB,
					LegacyUpdateTime: &timeB,
				},
			},
		}

		nextIndex++
		require.NoError(t, s.EnsureConfigEntry(nextIndex, updated.Clone(), nil))

		// check
		idx, raw, err = s.ConfigEntry(nil, structs.ServiceIntentions, "api", nil)
		require.NoError(t, err)
		require.Equal(t, nextIndex, idx)

		got, ok = raw.(*structs.ServiceIntentionsConfigEntry)
		require.True(t, ok)
		updated.RaftIndex = got.RaftIndex
		updated.Sources[0].LegacyCreateTime = &timeA // UpdateOver will not replace this
		require.Equal(t, updated, got)
	})
}

func TestStore_ConfigEntries(t *testing.T) {
	require := require.New(t)
	s := testConfigStateStore(t)

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

	require.NoError(s.EnsureConfigEntry(0, entry1, nil))
	require.NoError(s.EnsureConfigEntry(1, entry2, nil))
	require.NoError(s.EnsureConfigEntry(2, entry3, nil))

	// Get all entries
	idx, entries, err := s.ConfigEntries(nil, nil)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal([]structs.ConfigEntry{entry1, entry2, entry3}, entries)

	// Get all proxy entries
	idx, entries, err = s.ConfigEntriesByKind(nil, structs.ProxyDefaults, nil)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal([]structs.ConfigEntry{entry1}, entries)

	// Get all service entries
	ws := memdb.NewWatchSet()
	idx, entries, err = s.ConfigEntriesByKind(ws, structs.ServiceDefaults, nil)
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
	}, nil))
	require.True(watchFired(ws))
}

func TestStore_ConfigEntry_GraphValidation(t *testing.T) {
	type tcase struct {
		entries        []structs.ConfigEntry
		op             func(t *testing.T, s *Store) error
		expectErr      string
		expectGraphErr bool
	}
	cases := map[string]tcase{
		"splitter fails without default protocol": {
			entries: []structs.ConfigEntry{},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 100},
					},
				}
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"splitter fails with tcp protocol": {
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
						{Weight: 100},
					},
				}
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"splitter works with http protocol": {
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "tcp", // loses
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "main",
					Protocol:       "http",
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, ServiceSubset: "v1"},
						{Weight: 10, ServiceSubset: "v2"},
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				}
				return s.EnsureConfigEntry(0, entry, nil)
			},
		},
		"splitter works with http protocol (from proxy-defaults)": {
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, ServiceSubset: "v1"},
						{Weight: 10, ServiceSubset: "v2"},
					},
				}
				return s.EnsureConfigEntry(0, entry, nil)
			},
		},
		"router fails with tcp protocol": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"other": {
							Filter: "Service.Meta.version == other",
						},
					},
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
								ServiceSubset: "other",
							},
						},
					},
				}
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"router fails without default protocol": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"other": {
							Filter: "Service.Meta.version == other",
						},
					},
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
								ServiceSubset: "other",
							},
						},
					},
				}
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot remove default protocol after splitter created": {
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
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
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
			},
			op: func(t *testing.T, s *Store) error {
				return s.DeleteConfigEntry(0, structs.ServiceDefaults, "main", nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"cannot remove global default protocol after splitter created": {
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
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
			},
			op: func(t *testing.T, s *Store) error {
				return s.DeleteConfigEntry(0, structs.ProxyDefaults, structs.ProxyConfigGlobal, nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"can remove global default protocol after splitter created if service default overrides it": {
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
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
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
			},
			op: func(t *testing.T, s *Store) error {
				return s.DeleteConfigEntry(0, structs.ProxyDefaults, structs.ProxyConfigGlobal, nil)
			},
		},
		"cannot change to tcp protocol after splitter created": {
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
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
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
			},
			op: func(t *testing.T, s *Store) error {
				entry := &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				}
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"cannot remove default protocol after router created": {
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
						"other": {
							Filter: "Service.Meta.version == other",
						},
					},
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
								ServiceSubset: "other",
							},
						},
					},
				},
			},
			op: func(t *testing.T, s *Store) error {
				return s.DeleteConfigEntry(0, structs.ServiceDefaults, "main", nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"cannot change to tcp protocol after router created": {
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
						"other": {
							Filter: "Service.Meta.version == other",
						},
					},
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
								ServiceSubset: "other",
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
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot split to a service using tcp": {
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
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		"cannot route to a service using tcp": {
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
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot failover to a service using a different protocol": {
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
						"*": {
							Service: "other",
						},
					},
				}
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		"cannot redirect to a service using a different protocol": {
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
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"redirect to a subset that does exist is fine": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "other",
					ConnectTimeout: 33 * time.Second,
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
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
				return s.EnsureConfigEntry(0, entry, nil)
			},
		},
		"cannot redirect to a subset that does not exist": {
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
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      `does not have a subset named "v1"`,
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot introduce circular resolver redirect": {
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
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      `detected circular resolver redirect`,
			expectGraphErr: true,
		},
		"cannot introduce circular split": {
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
				return s.EnsureConfigEntry(0, entry, nil)
			},
			expectErr:      `detected circular reference`,
			expectGraphErr: true,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc

		t.Run(name, func(t *testing.T) {
			s := testConfigStateStore(t)
			for _, entry := range tc.entries {
				require.NoError(t, entry.Normalize())
				require.NoError(t, s.EnsureConfigEntry(0, entry, nil))
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
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil): nil,
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
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil): &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "grpc",
				},
			},
			expectAfter: []structs.ConfigEntryKindName{
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
			},
			checkAfter: func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries) {
				defaults := entrySet.GetService(structs.NewServiceID("main", nil))
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
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
				structs.NewConfigEntryKindName(structs.ServiceRouter, "main", nil),
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				structs.NewConfigEntryKindName(structs.ServiceRouter, "main", nil): nil,
			},
			expectAfter: []structs.ConfigEntryKindName{
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
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
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
				structs.NewConfigEntryKindName(structs.ServiceResolver, "main", nil),
				structs.NewConfigEntryKindName(structs.ServiceRouter, "main", nil),
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				structs.NewConfigEntryKindName(structs.ServiceRouter, "main", nil): &structs.ServiceRouterConfigEntry{
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
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
				structs.NewConfigEntryKindName(structs.ServiceResolver, "main", nil),
				structs.NewConfigEntryKindName(structs.ServiceRouter, "main", nil),
			},
			checkAfter: func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries) {
				router := entrySet.GetRouter(structs.NewServiceID("main", nil))
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
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
				structs.NewConfigEntryKindName(structs.ServiceSplitter, "main", nil),
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				structs.NewConfigEntryKindName(structs.ServiceSplitter, "main", nil): nil,
			},
			expectAfter: []structs.ConfigEntryKindName{
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
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
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
				structs.NewConfigEntryKindName(structs.ServiceSplitter, "main", nil),
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				structs.NewConfigEntryKindName(structs.ServiceSplitter, "main", nil): &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 85, ServiceSubset: "v1"},
						{Weight: 15, ServiceSubset: "v2"},
					},
				},
			},
			expectAfter: []structs.ConfigEntryKindName{
				structs.NewConfigEntryKindName(structs.ServiceDefaults, "main", nil),
				structs.NewConfigEntryKindName(structs.ServiceSplitter, "main", nil),
			},
			checkAfter: func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries) {
				splitter := entrySet.GetSplitter(structs.NewServiceID("main", nil))
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
				structs.NewConfigEntryKindName(structs.ServiceResolver, "main", nil),
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				structs.NewConfigEntryKindName(structs.ServiceResolver, "main", nil): nil,
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
				structs.NewConfigEntryKindName(structs.ServiceResolver, "main", nil),
			},
			overrides: map[structs.ConfigEntryKindName]structs.ConfigEntry{
				structs.NewConfigEntryKindName(structs.ServiceResolver, "main", nil): &structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "main",
					ConnectTimeout: 33 * time.Second,
				},
			},
			expectAfter: []structs.ConfigEntryKindName{
				structs.NewConfigEntryKindName(structs.ServiceResolver, "main", nil),
			},
			checkAfter: func(t *testing.T, entrySet *structs.DiscoveryChainConfigEntries) {
				resolver := entrySet.GetResolver(structs.NewServiceID("main", nil))
				require.NotNil(t, resolver)
				require.Equal(t, 33*time.Second, resolver.ConnectTimeout)
			},
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			s := testConfigStateStore(t)
			for _, entry := range tc.entries {
				require.NoError(t, s.EnsureConfigEntry(0, entry, nil))
			}

			t.Run("without override", func(t *testing.T) {
				_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", nil, nil)
				require.NoError(t, err)
				got := entrySetToKindNames(entrySet)
				require.ElementsMatch(t, tc.expectBefore, got)
			})

			t.Run("with override", func(t *testing.T) {
				_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", tc.overrides, nil)

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
		out = append(out, structs.NewConfigEntryKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	for _, entry := range entrySet.Splitters {
		out = append(out, structs.NewConfigEntryKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	for _, entry := range entrySet.Resolvers {
		out = append(out, structs.NewConfigEntryKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	for _, entry := range entrySet.Services {
		out = append(out, structs.NewConfigEntryKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	return out
}

func TestStore_ReadDiscoveryChainConfigEntries_SubsetSplit(t *testing.T) {
	s := testConfigStateStore(t)

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
				"v1": {
					Filter: "Service.Meta.version == v1",
				},
				"v2": {
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
		require.NoError(t, s.EnsureConfigEntry(0, entry, nil))
	}

	_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", nil, nil)
	require.NoError(t, err)

	require.Len(t, entrySet.Routers, 0)
	require.Len(t, entrySet.Splitters, 1)
	require.Len(t, entrySet.Resolvers, 1)
	require.Len(t, entrySet.Services, 1)
}

// TODO(rb): add ServiceIntentions tests

func TestStore_ValidateGatewayNamesCannotBeShared(t *testing.T) {
	s := testConfigStateStore(t)

	ingress := &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "gateway",
	}
	require.NoError(t, s.EnsureConfigEntry(0, ingress, nil))

	terminating := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "gateway",
	}
	// Cannot have 2 gateways with same service name
	require.Error(t, s.EnsureConfigEntry(1, terminating, nil))

	ingress = &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "gateway",
		Listeners: []structs.IngressListener{
			{Port: 8080},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(2, ingress, nil))
	require.NoError(t, s.DeleteConfigEntry(3, structs.IngressGateway, "gateway", nil))

	// Adding the terminating gateway with same name should now work
	require.NoError(t, s.EnsureConfigEntry(4, terminating, nil))

	// Cannot have 2 gateways with same service name
	require.Error(t, s.EnsureConfigEntry(5, ingress, nil))
}

func TestStore_ValidateIngressGatewayErrorOnMismatchedProtocols(t *testing.T) {
	newIngress := func(protocol, name string) *structs.IngressGatewayConfigEntry {
		return &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "gateway",
			Listeners: []structs.IngressListener{
				{
					Port:     8080,
					Protocol: protocol,
					Services: []structs.IngressService{
						{Name: name},
					},
				},
			},
		}
	}

	t.Run("http ingress fails with http upstream later changed to tcp", func(t *testing.T) {
		s := testConfigStateStore(t)

		// First set the target service as http
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}
		require.NoError(t, s.EnsureConfigEntry(0, expected, nil))

		// Next configure http ingress to route to the http service
		require.NoError(t, s.EnsureConfigEntry(1, newIngress("http", "web"), nil))

		t.Run("via modification", func(t *testing.T) {
			// Now redefine the target service as tcp
			expected = &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "web",
				Protocol: "tcp",
			}

			err := s.EnsureConfigEntry(2, expected, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), `has protocol "tcp"`)
		})
		t.Run("via deletion", func(t *testing.T) {
			// This will fall back to the default tcp.
			err := s.DeleteConfigEntry(2, structs.ServiceDefaults, "web", nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), `has protocol "tcp"`)
		})
	})

	t.Run("tcp ingress ok with tcp upstream (defaulted) later changed to http", func(t *testing.T) {
		s := testConfigStateStore(t)

		// First configure tcp ingress to route to a defaulted tcp service
		require.NoError(t, s.EnsureConfigEntry(0, newIngress("tcp", "web"), nil))

		// Now redefine the target service as http
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}
		require.NoError(t, s.EnsureConfigEntry(1, expected, nil))
	})

	t.Run("tcp ingress fails with tcp upstream (defaulted) later changed to http", func(t *testing.T) {
		s := testConfigStateStore(t)

		// First configure tcp ingress to route to a defaulted tcp service
		require.NoError(t, s.EnsureConfigEntry(0, newIngress("tcp", "web"), nil))

		// Now redefine the target service as http
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}
		require.NoError(t, s.EnsureConfigEntry(1, expected, nil))

		t.Run("and a router defined", func(t *testing.T) {
			// This part should fail.
			expected2 := &structs.ServiceRouterConfigEntry{
				Kind: structs.ServiceRouter,
				Name: "web",
			}
			err := s.EnsureConfigEntry(2, expected2, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), `has protocol "http"`)
		})

		t.Run("and a splitter defined", func(t *testing.T) {
			// This part should fail.
			expected2 := &structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "web",
				Splits: []structs.ServiceSplit{
					{Weight: 100},
				},
			}
			err := s.EnsureConfigEntry(2, expected2, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), `has protocol "http"`)
		})
	})

	t.Run("http ingress fails with tcp upstream (defaulted)", func(t *testing.T) {
		s := testConfigStateStore(t)
		err := s.EnsureConfigEntry(0, newIngress("http", "web"), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `has protocol "tcp"`)
	})

	t.Run("http ingress fails with http2 upstream (via proxy-defaults)", func(t *testing.T) {
		s := testConfigStateStore(t)
		expected := &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: "global",
			Config: map[string]interface{}{
				"protocol": "http2",
			},
		}
		require.NoError(t, s.EnsureConfigEntry(0, expected, nil))

		err := s.EnsureConfigEntry(1, newIngress("http", "web"), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `has protocol "http2"`)
	})

	t.Run("http ingress fails with grpc upstream (via service-defaults)", func(t *testing.T) {
		s := testConfigStateStore(t)
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "grpc",
		}
		require.NoError(t, s.EnsureConfigEntry(1, expected, nil))
		err := s.EnsureConfigEntry(2, newIngress("http", "web"), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `has protocol "grpc"`)
	})

	t.Run("http ingress ok with http upstream (via service-defaults)", func(t *testing.T) {
		s := testConfigStateStore(t)
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}
		require.NoError(t, s.EnsureConfigEntry(2, expected, nil))
		require.NoError(t, s.EnsureConfigEntry(3, newIngress("http", "web"), nil))
	})

	t.Run("http ingress ignores wildcard specifier", func(t *testing.T) {
		s := testConfigStateStore(t)
		require.NoError(t, s.EnsureConfigEntry(4, newIngress("http", "*"), nil))
	})

	t.Run("deleting ingress config entry ok", func(t *testing.T) {
		s := testConfigStateStore(t)
		require.NoError(t, s.EnsureConfigEntry(1, newIngress("tcp", "web"), nil))
		require.NoError(t, s.DeleteConfigEntry(5, structs.IngressGateway, "gateway", nil))
	})
}

func TestSourcesForTarget(t *testing.T) {
	defaultMeta := *structs.DefaultEnterpriseMeta()

	type expect struct {
		idx   uint64
		names []structs.ServiceName
	}
	tt := []struct {
		name    string
		entries []structs.ConfigEntry
		expect  expect
	}{
		{
			name:    "no relevant config entries",
			entries: []structs.ConfigEntry{},
			expect: expect{
				idx: 1,
				names: []structs.ServiceName{
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from route match",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "web",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/sink",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "sink",
							},
						},
					},
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from redirect",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from failover",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": {
							Service:     "sink",
							Datacenters: []string{"dc2", "dc3"},
						},
					},
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from splitter",
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
					Name: "web",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Service: "web"},
						{Weight: 10, Service: "sink"},
					},
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "chained route redirect",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "source",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/route",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "routed",
							},
						},
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "routed",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
			},
			expect: expect{
				idx: 3,
				names: []structs.ServiceName{
					{Name: "source", EnterpriseMeta: defaultMeta},
					{Name: "routed", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "kitchen sink with multiple services referencing sink directly",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "routed",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/sink",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "sink",
							},
						},
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "redirected",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "failed-over",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": {
							Service:     "sink",
							Datacenters: []string{"dc2", "dc3"},
						},
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "split",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Service: "no-op"},
						{Weight: 10, Service: "sink"},
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "unrelated",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Service: "zip"},
						{Weight: 10, Service: "zop"},
					},
				},
			},
			expect: expect{
				idx: 6,
				names: []structs.ServiceName{
					{Name: "split", EnterpriseMeta: defaultMeta},
					{Name: "failed-over", EnterpriseMeta: defaultMeta},
					{Name: "redirected", EnterpriseMeta: defaultMeta},
					{Name: "routed", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			ws := memdb.NewWatchSet()

			ca := &structs.CAConfiguration{
				Provider: "consul",
			}
			err := s.CASetConfig(0, ca)
			require.NoError(t, err)

			var i uint64 = 1
			for _, entry := range tc.entries {
				require.NoError(t, entry.Normalize())
				require.NoError(t, s.EnsureConfigEntry(i, entry, nil))
				i++
			}

			tx := s.db.ReadTxn()
			defer tx.Abort()

			sn := structs.NewServiceName("sink", structs.DefaultEnterpriseMeta())
			idx, names, err := s.discoveryChainSourcesTxn(tx, ws, "dc1", sn)
			require.NoError(t, err)

			require.Equal(t, tc.expect.idx, idx)
			require.ElementsMatch(t, tc.expect.names, names)
		})
	}
}

func TestTargetsForSource(t *testing.T) {
	defaultMeta := *structs.DefaultEnterpriseMeta()

	type expect struct {
		idx uint64
		ids []structs.ServiceName
	}
	tt := []struct {
		name    string
		entries []structs.ConfigEntry
		expect  expect
	}{
		{
			name: "from route match",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "web",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/sink",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "sink",
							},
						},
					},
				},
			},
			expect: expect{
				idx: 2,
				ids: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from redirect",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
			},
			expect: expect{
				idx: 2,
				ids: []structs.ServiceName{
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from failover",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": {
							Service:     "remote-web",
							Datacenters: []string{"dc2", "dc3"},
						},
					},
				},
			},
			expect: expect{
				idx: 2,
				ids: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from splitter",
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
					Name: "web",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Service: "web"},
						{Weight: 10, Service: "sink"},
					},
				},
			},
			expect: expect{
				idx: 2,
				ids: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "chained route redirect",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "web",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/route",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "routed",
							},
						},
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "routed",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
			},
			expect: expect{
				idx: 3,
				ids: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			ws := memdb.NewWatchSet()

			ca := &structs.CAConfiguration{
				Provider: "consul",
			}
			err := s.CASetConfig(0, ca)
			require.NoError(t, err)

			var i uint64 = 1
			for _, entry := range tc.entries {
				require.NoError(t, entry.Normalize())
				require.NoError(t, s.EnsureConfigEntry(i, entry, nil))
				i++
			}

			tx := s.db.ReadTxn()
			defer tx.Abort()

			idx, ids, err := s.discoveryChainTargetsTxn(tx, ws, "dc1", "web", nil)
			require.NoError(t, err)

			require.Equal(t, tc.expect.idx, idx)
			require.ElementsMatch(t, tc.expect.ids, ids)
		})
	}
}

func TestStore_ValidateServiceIntentionsErrorOnIncompatibleProtocols(t *testing.T) {
	l7perms := []*structs.IntentionPermission{
		{
			Action: structs.IntentionActionAllow,
			HTTP: &structs.IntentionHTTPPermission{
				PathPrefix: "/v2/",
			},
		},
	}

	serviceDefaults := func(service, protocol string) *structs.ServiceConfigEntry {
		return &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     service,
			Protocol: protocol,
		}
	}

	proxyDefaults := func(protocol string) *structs.ProxyConfigEntry {
		return &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": protocol,
			},
		}
	}

	type operation struct {
		entry    structs.ConfigEntry
		deletion bool
	}

	type testcase struct {
		ops           []operation
		expectLastErr string
	}

	cases := map[string]testcase{
		"L4 intention cannot upgrade to L7 when tcp": {
			ops: []operation{
				{ // set the target service as tcp
					entry: serviceDefaults("api", "tcp"),
				},
				{ // create an L4 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Action: structs.IntentionActionAllow},
						},
					},
				},
				{ // Should fail if converted to L7
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
		"L4 intention can upgrade to L7 when made http via service-defaults": {
			ops: []operation{
				{ // set the target service as tcp
					entry: serviceDefaults("api", "tcp"),
				},
				{ // create an L4 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Action: structs.IntentionActionAllow},
						},
					},
				},
				{ // set the target service as http
					entry: serviceDefaults("api", "http"),
				},
				{ // Should succeed if converted to L7
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
			},
		},
		"L4 intention can upgrade to L7 when made http via proxy-defaults": {
			ops: []operation{
				{ // set the target service as tcp
					entry: proxyDefaults("tcp"),
				},
				{ // create an L4 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Action: structs.IntentionActionAllow},
						},
					},
				},
				{ // set the target service as http
					entry: proxyDefaults("http"),
				},
				{ // Should succeed if converted to L7
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
			},
		},
		"L7 intention cannot have protocol downgraded to tcp via modification via service-defaults": {
			ops: []operation{
				{ // set the target service as http
					entry: serviceDefaults("api", "http"),
				},
				{ // create an L7 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
				{ // setting the target service as tcp should fail
					entry: serviceDefaults("api", "tcp"),
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
		"L7 intention cannot have protocol downgraded to tcp via modification via proxy-defaults": {
			ops: []operation{
				{ // set the target service as http
					entry: proxyDefaults("http"),
				},
				{ // create an L7 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
				{ // setting the target service as tcp should fail
					entry: proxyDefaults("tcp"),
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
		"L7 intention cannot have protocol downgraded to tcp via deletion of service-defaults": {
			ops: []operation{
				{ // set the target service as http
					entry: serviceDefaults("api", "http"),
				},
				{ // create an L7 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
				{ // setting the target service as tcp should fail
					entry:    serviceDefaults("api", "tcp"),
					deletion: true,
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
		"L7 intention cannot have protocol downgraded to tcp via deletion of proxy-defaults": {
			ops: []operation{
				{ // set the target service as http
					entry: proxyDefaults("http"),
				},
				{ // create an L7 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
				{ // setting the target service as tcp should fail
					entry:    proxyDefaults("tcp"),
					deletion: true,
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			s := testStateStore(t)

			var nextIndex = uint64(1)

			for i, op := range tc.ops {
				isLast := (i == len(tc.ops)-1)

				var err error
				if op.deletion {
					err = s.DeleteConfigEntry(nextIndex, op.entry.GetKind(), op.entry.GetName(), nil)
				} else {
					err = s.EnsureConfigEntry(nextIndex, op.entry, nil)
				}

				if isLast && tc.expectLastErr != "" {
					testutil.RequireErrorContains(t, err, `has protocol "tcp"`)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}
