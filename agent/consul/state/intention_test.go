// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

var (
	testLocation = time.FixedZone("UTC-8", -8*60*60)

	testTimeA = time.Date(1955, 11, 5, 6, 15, 0, 0, testLocation)
	testTimeB = time.Date(1985, 10, 26, 1, 35, 0, 0, testLocation)
	testTimeC = time.Date(2015, 10, 21, 16, 29, 0, 0, testLocation)
)

func testBothIntentionFormats(t *testing.T, f func(t *testing.T, s *Store, legacy bool)) {
	t.Helper()

	// Within the body of the callback, only use Legacy CRUD functions to edit
	// data (pivoting on the legacy flag), and exclusively use the generic
	// functions that flip-flop between tables to read it.

	t.Run("legacy", func(t *testing.T) {
		// NOTE: This one tests that the old state machine functions still do
		// what we expect. No newly initiated user edits should go through
		// these paths, just lingering raft log entries from before the upgrade
		// to 1.9.0.
		s := testStateStore(t)
		f(t, s, true)
	})

	t.Run("config-entries", func(t *testing.T) {
		s := testConfigStateStore(t)
		f(t, s, false)
	})
}

func TestStore_IntentionGet_none(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {

		// Querying with no results returns nil.
		ws := memdb.NewWatchSet()
		idx, _, res, err := s.IntentionGet(ws, testUUID())
		assert.Equal(t, uint64(1), idx)
		assert.Nil(t, res)
		assert.Nil(t, err)
	})
}

func TestStore_IntentionSetGet_basic(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		lastIndex := uint64(1)

		// Call Get to populate the watch set
		ws := memdb.NewWatchSet()
		_, _, _, err := s.IntentionGet(ws, testUUID())
		require.Nil(t, err)

		// Build a valid intention
		var (
			legacyIxn   *structs.Intention
			configEntry *structs.ServiceIntentionsConfigEntry

			expected *structs.Intention
		)
		if legacy {
			legacyIxn = &structs.Intention{
				ID:              testUUID(),
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "web",
				Meta:            map[string]string{},
				CreatedAt:       testTimeA,
				UpdatedAt:       testTimeA,
			}

			// Inserting a with empty ID is disallowed.
			lastIndex++
			require.NoError(t, s.LegacyIntentionSet(lastIndex, legacyIxn))

			// Make sure the right index got updated.
			require.Equal(t, lastIndex, s.maxIndex(tableConnectIntentions))
			require.Equal(t, uint64(0), s.maxIndex(tableConfigEntries))

			expected = &structs.Intention{
				ID:              legacyIxn.ID,
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "web",
				Meta:            map[string]string{},
				CreatedAt:       testTimeA,
				UpdatedAt:       testTimeA,
				RaftIndex: structs.RaftIndex{
					CreateIndex: lastIndex,
					ModifyIndex: lastIndex,
				},
			}
			//nolint:staticcheck
			expected.UpdatePrecedence()
		} else {
			srcID := testUUID()
			configEntry = &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "web",
				Sources: []*structs.SourceIntention{
					{
						LegacyID:         srcID,
						Name:             "*",
						Action:           structs.IntentionActionAllow,
						LegacyMeta:       map[string]string{},
						LegacyCreateTime: &testTimeA,
						LegacyUpdateTime: &testTimeA,
					},
				},
			}

			lastIndex++
			require.NoError(t, configEntry.LegacyNormalize())
			require.NoError(t, configEntry.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(lastIndex, configEntry.Clone()))

			// Make sure the config entry index got updated instead of the old intentions one
			require.Equal(t, lastIndex, s.maxIndex(tableConfigEntries))
			require.Equal(t, uint64(0), s.maxIndex(tableConnectIntentions))

			expected = &structs.Intention{
				ID:              srcID,
				SourceNS:        "default",
				SourceName:      "*",
				SourceType:      structs.IntentionSourceConsul,
				DestinationNS:   "default",
				DestinationName: "web",
				Meta:            map[string]string{},
				Action:          structs.IntentionActionAllow,
				RaftIndex: structs.RaftIndex{
					CreateIndex: lastIndex,
					ModifyIndex: lastIndex,
				},
				CreatedAt: *configEntry.Sources[0].LegacyCreateTime,
				UpdatedAt: *configEntry.Sources[0].LegacyUpdateTime,
			}
			//nolint:staticcheck
			expected.UpdatePrecedence()
			//nolint:staticcheck
			expected.SetHash()

			expected.FillPartitionAndNamespace(nil, true)
		}
		require.True(t, watchFired(ws), "watch fired")

		// Read it back out and verify it.
		ws = memdb.NewWatchSet()
		idx, _, actual, err := s.IntentionGet(ws, expected.ID)
		require.NoError(t, err)
		require.Equal(t, expected.CreateIndex, idx)
		require.Equal(t, expected, actual)

		if legacy {
			// Change a value and test updating
			legacyIxn.SourceNS = "foo"
			lastIndex++
			require.NoError(t, s.LegacyIntentionSet(lastIndex, legacyIxn))

			// Change a value that isn't in the unique 4 tuple and check we don't
			// incorrectly consider this a duplicate when updating.
			legacyIxn.Action = structs.IntentionActionDeny
			lastIndex++
			require.NoError(t, s.LegacyIntentionSet(lastIndex, legacyIxn))

			// Make sure the index got updated.
			require.Equal(t, lastIndex, s.maxIndex(tableConnectIntentions))
			require.Equal(t, uint64(0), s.maxIndex(tableConfigEntries))

			expected.SourceNS = legacyIxn.SourceNS
			expected.Action = structs.IntentionActionDeny
			expected.ModifyIndex = lastIndex

		} else {
			// Change a value and test updating
			configEntry.Sources[0].Description = "test-desc1"
			lastIndex++
			require.NoError(t, configEntry.LegacyNormalize())
			require.NoError(t, configEntry.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(lastIndex, configEntry.Clone()))

			// Change a value that isn't in the unique 4 tuple and check we don't
			// incorrectly consider this a duplicate when updating.
			configEntry.Sources[0].Action = structs.IntentionActionDeny
			lastIndex++
			require.NoError(t, configEntry.LegacyNormalize())
			require.NoError(t, configEntry.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(lastIndex, configEntry.Clone()))

			// Make sure the config entry index got updated instead of the old intentions one
			require.Equal(t, lastIndex, s.maxIndex(tableConfigEntries))
			require.Equal(t, uint64(0), s.maxIndex(tableConnectIntentions))

			expected.Description = configEntry.Sources[0].Description
			expected.Action = structs.IntentionActionDeny
			expected.UpdatedAt = *configEntry.Sources[0].LegacyUpdateTime
			expected.ModifyIndex = lastIndex
			//nolint:staticcheck
			expected.UpdatePrecedence()
			//nolint:staticcheck
			expected.SetHash()
		}
		require.True(t, watchFired(ws), "watch fired")

		// Read it back and verify the data was updated
		ws = memdb.NewWatchSet()
		idx, _, actual, err = s.IntentionGet(ws, expected.ID)
		require.NoError(t, err)
		require.Equal(t, expected.ModifyIndex, idx)
		require.Equal(t, expected, actual)

		if legacy {
			// Attempt to insert another intention with duplicate 4-tuple
			legacyIxn = &structs.Intention{
				ID:              testUUID(),
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "web",
				Meta:            map[string]string{},
			}

			// Duplicate 4-tuple should cause an error
			ws = memdb.NewWatchSet()
			lastIndex++
			require.Error(t, s.LegacyIntentionSet(lastIndex, legacyIxn))

			// Make sure the index did NOT get updated.
			require.Equal(t, lastIndex-1, s.maxIndex(tableConnectIntentions))
			require.Equal(t, uint64(0), s.maxIndex(tableConfigEntries))
			require.False(t, watchFired(ws), "watch not fired")
		}
	})
}

func TestStore_LegacyIntentionSet_failsAfterUpgrade(t *testing.T) {
	// note: special case test doesn't need variants
	s := testConfigStateStore(t)

	ixn := structs.Intention{
		ID:              testUUID(),
		SourceNS:        "default",
		SourceName:      "*",
		DestinationNS:   "default",
		DestinationName: "web",
		Action:          structs.IntentionActionAllow,
		Meta:            map[string]string{},
	}

	err := s.LegacyIntentionSet(1, &ixn)
	testutil.RequireErrorContains(t, err, ErrLegacyIntentionsAreDisabled.Error())
}

func TestStore_LegacyIntentionDelete_failsAfterUpgrade(t *testing.T) {
	// note: special case test doesn't need variants
	s := testConfigStateStore(t)

	err := s.LegacyIntentionDelete(1, testUUID())
	testutil.RequireErrorContains(t, err, ErrLegacyIntentionsAreDisabled.Error())
}

func TestStore_IntentionMutation(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		if legacy {
			mut := &structs.IntentionMutation{}
			err := s.IntentionMutation(1, structs.IntentionOpCreate, mut)
			testutil.RequireErrorContains(t, err, "state: IntentionMutation() is not allowed when intentions are not stored in config entries")
		} else {
			testStore_IntentionMutation(t, s)
		}
	})
}

func testStore_IntentionMutation(t *testing.T, s *Store) {
	lastIndex := uint64(1)

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	var (
		id1 = testUUID()
		id2 = testUUID()
		id3 = testUUID()
	)

	eqEntry := func(t *testing.T, expect, got *structs.ServiceIntentionsConfigEntry) {
		t.Helper()

		// Zero out some fields for comparison.
		got = got.Clone()
		got.RaftIndex = structs.RaftIndex{}
		for _, src := range got.Sources {
			src.LegacyCreateTime = nil
			src.LegacyUpdateTime = nil
			if len(src.LegacyMeta) == 0 {
				src.LegacyMeta = nil
			}
		}
		require.Equal(t, expect, got)
	}

	// Try to create an intention without an ID to prove that LegacyValidate is being called.
	testutil.RequireErrorContains(t, s.IntentionMutation(lastIndex, structs.IntentionOpCreate, &structs.IntentionMutation{
		Destination: structs.NewServiceName("api", defaultEntMeta),
		Value: &structs.SourceIntention{
			Name:             "web",
			EnterpriseMeta:   *defaultEntMeta,
			Action:           structs.IntentionActionAllow,
			LegacyCreateTime: &testTimeA,
			LegacyUpdateTime: &testTimeA,
		},
	}), `Sources[0].LegacyID must be set`)

	// Create intention and create config entry
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpCreate, &structs.IntentionMutation{
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Value: &structs.SourceIntention{
				Name:             "web",
				EnterpriseMeta:   *defaultEntMeta,
				Action:           structs.IntentionActionAllow,
				LegacyID:         id1,
				LegacyCreateTime: &testTimeA,
				LegacyUpdateTime: &testTimeA,
			},
		}))

		// Ensure it's there now.
		idx, entry, ixn, err := s.IntentionGet(nil, id1)
		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NotNil(t, ixn)
		require.Equal(t, lastIndex, idx)

		eqEntry(t, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "api",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					LegacyID:       id1,
					Name:           "web",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
			},
		}, entry)

		// only one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		lastIndex++
	}

	// Try to create a duplicate intention.
	{
		testutil.RequireErrorContains(t, s.IntentionMutation(lastIndex, structs.IntentionOpCreate, &structs.IntentionMutation{
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Value: &structs.SourceIntention{
				Name:             "web",
				EnterpriseMeta:   *defaultEntMeta,
				Action:           structs.IntentionActionDeny,
				LegacyID:         id2,
				LegacyCreateTime: &testTimeB,
				LegacyUpdateTime: &testTimeB,
			},
		}), `more than once`)
	}

	// Create intention with existing config entry
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpCreate, &structs.IntentionMutation{
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Value: &structs.SourceIntention{
				Name:             "debug",
				EnterpriseMeta:   *defaultEntMeta,
				Action:           structs.IntentionActionDeny,
				LegacyID:         id2,
				LegacyCreateTime: &testTimeB,
				LegacyUpdateTime: &testTimeB,
			},
		}))

		// Ensure it's there now.
		idx, entry, ixn, err := s.IntentionGet(nil, id2)
		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NotNil(t, ixn)
		require.Equal(t, lastIndex, idx)

		eqEntry(t, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "api",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "web",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					LegacyID:       id1,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
				{
					Name:           "debug",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionDeny,
					LegacyID:       id2,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
			},
		}, entry)

		// only one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		lastIndex++
	}

	// Try to update an intention without specifying an ID
	testutil.RequireErrorContains(t, s.IntentionMutation(lastIndex, structs.IntentionOpUpdate, &structs.IntentionMutation{
		ID:          "",
		Destination: structs.NewServiceName("api", defaultEntMeta),
		Value: &structs.SourceIntention{
			Name:           "web",
			EnterpriseMeta: *defaultEntMeta,
			Action:         structs.IntentionActionAllow,
		},
	}), `failed config entry lookup: index error: UUID must be 36 characters`)

	// Try to update a non-existent intention
	testutil.RequireErrorContains(t, s.IntentionMutation(lastIndex, structs.IntentionOpUpdate, &structs.IntentionMutation{
		ID:          id3,
		Destination: structs.NewServiceName("api", defaultEntMeta),
		Value: &structs.SourceIntention{
			Name:           "web",
			EnterpriseMeta: *defaultEntMeta,
			Action:         structs.IntentionActionAllow,
		},
	}), `Cannot modify non-existent intention`)

	// Update an existing intention by ID
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpUpdate, &structs.IntentionMutation{
			ID:          id2,
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Value: &structs.SourceIntention{
				Name:             "debug",
				EnterpriseMeta:   *defaultEntMeta,
				Action:           structs.IntentionActionDeny,
				LegacyID:         id2,
				LegacyCreateTime: &testTimeB,
				LegacyUpdateTime: &testTimeC,
				Description:      "op update",
			},
		}))

		// Ensure it's there now.
		idx, entry, ixn, err := s.IntentionGet(nil, id2)
		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NotNil(t, ixn)
		require.Equal(t, lastIndex, idx)

		eqEntry(t, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "api",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "web",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					LegacyID:       id1,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
				{
					Name:           "debug",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionDeny,
					LegacyID:       id2,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
					Description:    "op update",
				},
			},
		}, entry)

		// only one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		lastIndex++
	}

	// Try to delete a non-existent intention
	testutil.RequireErrorContains(t, s.IntentionMutation(lastIndex, structs.IntentionOpDelete, &structs.IntentionMutation{
		ID: id3,
	}), `Cannot delete non-existent intention`)

	// delete by id
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpDelete, &structs.IntentionMutation{
			ID: id1,
		}))

		// only one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		eqEntry(t, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "api",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "debug",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionDeny,
					LegacyID:       id2,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
					Description:    "op update",
				},
			},
		}, entries[0].(*structs.ServiceIntentionsConfigEntry))

		lastIndex++
	}

	// delete last one by id
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpDelete, &structs.IntentionMutation{
			ID: id2,
		}))

		// none one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Empty(t, entries)

		lastIndex++
	}

	// upsert intention for first time
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpUpsert, &structs.IntentionMutation{
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Value: &structs.SourceIntention{
				Name:           "web",
				EnterpriseMeta: *defaultEntMeta,
				Action:         structs.IntentionActionAllow,
			},
		}))

		// Ensure it's there now.
		idx, entry, ixn, err := s.IntentionGetExact(nil, &structs.IntentionQueryExact{
			SourceNS:        "default",
			SourceName:      "web",
			DestinationNS:   "default",
			DestinationName: "api",
		})
		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NotNil(t, ixn)
		require.Equal(t, lastIndex, idx)

		eqEntry(t, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "api",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "web",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
			},
		}, entry)

		// only one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		lastIndex++
	}

	// upsert over itself (REPLACE)
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpUpsert, &structs.IntentionMutation{
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Source:      structs.NewServiceName("web", defaultEntMeta),
			Value: &structs.SourceIntention{
				Name:           "web",
				EnterpriseMeta: *defaultEntMeta,
				Action:         structs.IntentionActionAllow,
				Description:    "upserted over",
			},
		}))

		// Ensure it's there now.
		idx, entry, ixn, err := s.IntentionGetExact(nil, &structs.IntentionQueryExact{
			SourceNS:        "default",
			SourceName:      "web",
			DestinationNS:   "default",
			DestinationName: "api",
		})
		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NotNil(t, ixn)
		require.Equal(t, lastIndex, idx)
		require.Equal(t, "upserted over", ixn.Description)

		eqEntry(t, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "api",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "web",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
					Description:    "upserted over",
				},
			},
		}, entry)

		// only one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		lastIndex++
	}

	// upsert into existing config entry (APPEND)
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpUpsert, &structs.IntentionMutation{
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Source:      structs.NewServiceName("debug", defaultEntMeta),
			Value: &structs.SourceIntention{
				Name:           "debug",
				EnterpriseMeta: *defaultEntMeta,
				Action:         structs.IntentionActionDeny,
			},
		}))

		// Ensure it's there now.
		idx, entry, ixn, err := s.IntentionGetExact(nil, &structs.IntentionQueryExact{
			SourceNS:        "default",
			SourceName:      "debug",
			DestinationNS:   "default",
			DestinationName: "api",
		})
		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NotNil(t, ixn)
		require.Equal(t, lastIndex, idx)

		eqEntry(t, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "api",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "web",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
					Description:    "upserted over",
				},
				{
					Name:           "debug",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionDeny,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
			},
		}, entry)

		// only one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		lastIndex++
	}

	// Try to delete a non-existent intention by name
	require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpDelete, &structs.IntentionMutation{
		Destination: structs.NewServiceName("api", defaultEntMeta),
		Source:      structs.NewServiceName("blurb", defaultEntMeta),
	}))

	// delete by name
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpDelete, &structs.IntentionMutation{
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Source:      structs.NewServiceName("web", defaultEntMeta),
		}))

		// only one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		eqEntry(t, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "api",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "debug",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionDeny,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
			},
		}, entries[0].(*structs.ServiceIntentionsConfigEntry))

		lastIndex++
	}

	// delete last one by name
	{
		require.NoError(t, s.IntentionMutation(lastIndex, structs.IntentionOpDelete, &structs.IntentionMutation{
			Destination: structs.NewServiceName("api", defaultEntMeta),
			Source:      structs.NewServiceName("debug", defaultEntMeta),
		}))

		// none one
		_, entries, err := s.ConfigEntries(nil, nil)
		require.NoError(t, err)
		require.Empty(t, entries)

		lastIndex++
	}

	// Try to update an intention with an ID on a non-legacy config entry.
	{
		idFake := testUUID()

		require.NoError(t, s.EnsureConfigEntry(lastIndex, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "new",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "web",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
			},
		}))

		lastIndex++

		// ...via create
		testutil.RequireErrorContains(t, s.IntentionMutation(lastIndex, structs.IntentionOpCreate, &structs.IntentionMutation{
			Destination: structs.NewServiceName("new", defaultEntMeta),
			Value: &structs.SourceIntention{
				Name:           "old",
				EnterpriseMeta: *defaultEntMeta,
				Action:         structs.IntentionActionAllow,
				LegacyID:       idFake,
			},
		}), `cannot use legacy intention API to edit intentions with a destination`)
	}
}

func TestStore_LegacyIntentionSet_emptyId(t *testing.T) {
	// note: irrelevant test for config entries variant
	s := testStateStore(t)

	ws := memdb.NewWatchSet()
	_, _, _, err := s.IntentionGet(ws, testUUID())
	require.NoError(t, err)

	// Inserting a with empty ID is disallowed.
	err = s.LegacyIntentionSet(1, &structs.Intention{})
	require.Error(t, err)
	require.Contains(t, err.Error(), ErrMissingIntentionID.Error())

	// Index is not updated if nothing is saved.
	require.Equal(t, s.maxIndex(tableConnectIntentions), uint64(0))
	require.Equal(t, uint64(0), s.maxIndex(tableConfigEntries))

	require.False(t, watchFired(ws), "watch fired")
}

func TestStore_IntentionSet_updateCreatedAt(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		// Build a valid intention
		var (
			id = testUUID()
		)

		if legacy {
			ixn := structs.Intention{
				ID:              id,
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "web",
				Action:          structs.IntentionActionAllow,
				CreatedAt:       testTimeA,
				UpdatedAt:       testTimeA,
			}

			// Insert
			require.NoError(t, s.LegacyIntentionSet(1, &ixn))

			// Change a value and test updating
			ixnUpdate := ixn
			ixnUpdate.CreatedAt = testTimeB
			require.NoError(t, s.LegacyIntentionSet(2, &ixnUpdate))

			id = ixn.ID

		} else {
			conf := &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "web",
				Sources: []*structs.SourceIntention{
					{
						LegacyID:         id,
						Name:             "*",
						Action:           structs.IntentionActionAllow,
						LegacyMeta:       map[string]string{},
						LegacyCreateTime: &testTimeA,
						LegacyUpdateTime: &testTimeA,
					},
				},
			}

			require.NoError(t, conf.LegacyNormalize())
			require.NoError(t, conf.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(1, conf.Clone()))
		}

		// Read it back and verify
		_, _, actual, err := s.IntentionGet(nil, id)
		require.NoError(t, err)
		require.NotNil(t, actual)
		require.Equal(t, testTimeA, actual.CreatedAt)
	})
}

func TestStore_IntentionSet_metaNil(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		id := testUUID()
		if legacy {
			// Build a valid intention
			ixn := &structs.Intention{
				ID:              id,
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "web",
				Action:          structs.IntentionActionAllow,
				CreatedAt:       testTimeA,
				UpdatedAt:       testTimeA,
			}

			// Insert
			require.NoError(t, s.LegacyIntentionSet(1, ixn))
		} else {
			// Build a valid intention
			conf := &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "web",
				Sources: []*structs.SourceIntention{
					{
						LegacyID:         id,
						Name:             "*",
						Action:           structs.IntentionActionAllow,
						LegacyCreateTime: &testTimeA,
						LegacyUpdateTime: &testTimeA,
					},
				},
			}

			// Insert
			require.NoError(t, conf.LegacyNormalize())
			require.NoError(t, conf.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(1, conf.Clone()))
		}

		// Read it back and verify
		_, _, actual, err := s.IntentionGet(nil, id)
		require.NoError(t, err)
		require.NotNil(t, actual.Meta)
	})
}

func TestStore_IntentionSet_metaSet(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		var (
			id         = testUUID()
			expectMeta = map[string]string{"foo": "bar"}
		)
		if legacy {
			// Build a valid intention
			ixn := structs.Intention{
				ID:              id,
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "web",
				Action:          structs.IntentionActionAllow,
				CreatedAt:       testTimeA,
				UpdatedAt:       testTimeA,
				Meta:            expectMeta,
			}

			// Insert
			require.NoError(t, s.LegacyIntentionSet(1, &ixn))

		} else {
			// Build a valid intention
			conf := &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "web",
				Sources: []*structs.SourceIntention{
					{
						LegacyID:         id,
						Name:             "*",
						Action:           structs.IntentionActionAllow,
						LegacyCreateTime: &testTimeA,
						LegacyUpdateTime: &testTimeA,
						LegacyMeta:       expectMeta,
					},
				},
			}

			// Insert
			require.NoError(t, conf.LegacyNormalize())
			require.NoError(t, conf.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(1, conf.Clone()))
		}

		// Read it back and verify
		_, _, actual, err := s.IntentionGet(nil, id)
		require.NoError(t, err)
		require.Equal(t, expectMeta, actual.Meta)
	})
}

func TestStore_IntentionDelete(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		lastIndex := uint64(1)

		// Call Get to populate the watch set
		ws := memdb.NewWatchSet()
		_, _, _, err := s.IntentionGet(ws, testUUID())
		require.NoError(t, err)

		id := testUUID()
		// Create
		if legacy {
			ixn := &structs.Intention{
				ID:              id,
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "web",
				Action:          structs.IntentionActionAllow,
				CreatedAt:       testTimeA,
				UpdatedAt:       testTimeA,
			}
			lastIndex++
			require.NoError(t, s.LegacyIntentionSet(lastIndex, ixn))

			// Make sure the index got updated.
			require.Equal(t, s.maxIndex(tableConnectIntentions), lastIndex)
			require.Equal(t, uint64(0), s.maxIndex(tableConfigEntries))
		} else {
			conf := &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "web",
				Sources: []*structs.SourceIntention{
					{
						LegacyID:         id,
						Name:             "*",
						Action:           structs.IntentionActionAllow,
						LegacyCreateTime: &testTimeA,
						LegacyUpdateTime: &testTimeA,
					},
				},
			}

			// Insert
			require.NoError(t, conf.LegacyNormalize())
			require.NoError(t, conf.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(1, conf.Clone()))

			// Make sure the index got updated.
			require.Equal(t, s.maxIndex(tableConfigEntries), lastIndex)
			require.Equal(t, uint64(0), s.maxIndex(tableConnectIntentions))
		}
		require.True(t, watchFired(ws), "watch fired")

		// Sanity check to make sure it's there.
		idx, _, actual, err := s.IntentionGet(nil, id)
		require.NoError(t, err)
		require.Equal(t, idx, lastIndex)
		require.NotNil(t, actual)

		// Delete
		if legacy {
			lastIndex++
			require.NoError(t, s.LegacyIntentionDelete(lastIndex, id))

			// Make sure the index got updated.
			require.Equal(t, s.maxIndex(tableConnectIntentions), lastIndex)
			require.Equal(t, uint64(0), s.maxIndex(tableConfigEntries))
		} else {
			lastIndex++
			require.NoError(t, s.DeleteConfigEntry(lastIndex, structs.ServiceIntentions, "web", nil))

			// Make sure the index got updated.
			require.Equal(t, s.maxIndex(tableConfigEntries), lastIndex)
			require.Equal(t, uint64(0), s.maxIndex(tableConnectIntentions))
		}
		require.True(t, watchFired(ws), "watch fired")

		// Sanity check to make sure it's not there.
		idx, _, actual, err = s.IntentionGet(nil, id)
		require.NoError(t, err)
		require.Equal(t, idx, lastIndex)
		require.Nil(t, actual)
	})
}

func TestStore_IntentionsList(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		lastIndex := uint64(0)
		if legacy {
			lastIndex = 1 // minor state machine implementation difference
		}

		entMeta := structs.WildcardEnterpriseMetaInDefaultPartition()

		// Querying with no results returns nil.
		ws := memdb.NewWatchSet()
		idx, res, fromConfig, err := s.Intentions(ws, entMeta)
		require.NoError(t, err)
		require.Equal(t, !legacy, fromConfig)
		require.Nil(t, res)
		require.Equal(t, lastIndex, idx)

		testIntention := func(src, dst string) *structs.Intention {
			ret := &structs.Intention{
				ID:              testUUID(),
				SourceNS:        "default",
				SourceName:      src,
				DestinationNS:   "default",
				DestinationName: dst,
				SourceType:      structs.IntentionSourceConsul,
				Action:          structs.IntentionActionAllow,
				Meta:            map[string]string{},
				CreatedAt:       testTimeA,
				UpdatedAt:       testTimeA,
			}
			if !legacy {
				ret.FillPartitionAndNamespace(nil, true)
			}
			return ret
		}

		testConfigEntry := func(dst string, srcs ...string) *structs.ServiceIntentionsConfigEntry {
			conf := &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: dst,
			}
			id := testUUID()
			for _, src := range srcs {
				conf.Sources = append(conf.Sources, &structs.SourceIntention{
					LegacyID:         id,
					Name:             src,
					Action:           structs.IntentionActionAllow,
					LegacyCreateTime: &testTimeA,
					LegacyUpdateTime: &testTimeA,
				})
			}
			return conf
		}

		clearIrrelevantFields := func(ixns ...*structs.Intention) {
			// Clear fields irrelevant for comparison.
			for _, ixn := range ixns {
				ixn.Hash = nil
				ixn.CreateIndex = 0
				ixn.ModifyIndex = 0
				ixn.CreatedAt = time.Time{}
				ixn.UpdatedAt = time.Time{}
			}
		}

		cmpIntention := func(ixn *structs.Intention, id string) *structs.Intention {
			ixn2 := ixn.Clone()
			ixn2.ID = id
			clearIrrelevantFields(ixn2)
			//nolint:staticcheck
			ixn2.UpdatePrecedence()
			return ixn2
		}

		var (
			expectIDs []string
		)

		// Create some intentions
		if legacy {
			ixns := structs.Intentions{
				testIntention("foo", "bar"),
				testIntention("*", "bar"),
				testIntention("foo", "*"),
				testIntention("*", "*"),
			}

			for _, ixn := range ixns {
				expectIDs = append(expectIDs, ixn.ID)
				lastIndex++
				require.NoError(t, s.LegacyIntentionSet(lastIndex, ixn))
			}

		} else {
			confs := []*structs.ServiceIntentionsConfigEntry{
				testConfigEntry("bar", "foo", "*"),
				testConfigEntry("*", "foo", "*"),
			}

			for _, conf := range confs {
				require.NoError(t, conf.LegacyNormalize())
				require.NoError(t, conf.LegacyValidate())
				lastIndex++
				require.NoError(t, s.EnsureConfigEntry(lastIndex, conf))
			}

			expectIDs = []string{
				confs[0].Sources[0].LegacyID, // foo->bar
				confs[0].Sources[1].LegacyID, // *->bar
				confs[1].Sources[0].LegacyID, // foo->*
				confs[1].Sources[1].LegacyID, // *->*
			}
		}
		require.True(t, watchFired(ws), "watch fired")

		// Read it back and verify.
		expected := structs.Intentions{
			cmpIntention(testIntention("foo", "bar"), expectIDs[0]),
			cmpIntention(testIntention("*", "bar"), expectIDs[1]),
			cmpIntention(testIntention("foo", "*"), expectIDs[2]),
			cmpIntention(testIntention("*", "*"), expectIDs[3]),
		}

		idx, actual, fromConfig, err := s.Intentions(nil, entMeta)
		require.NoError(t, err)
		require.Equal(t, !legacy, fromConfig)
		require.Equal(t, lastIndex, idx)

		clearIrrelevantFields(actual...)
		require.Equal(t, expected, actual)
	})
}

// TestStore_IntentionExact_ConfigEntries test that we can get a local config entry intention
// and a peered config entry intention with an IntentionGetExact call
func TestStore_IntentionExact_ConfigEntries(t *testing.T) {
	s := testConfigStateStore(t)
	inputs := []*structs.ServiceIntentionsConfigEntry{
		{
			Kind: structs.ServiceIntentions,
			Name: "foo",
			Sources: []*structs.SourceIntention{
				{
					Action:      structs.IntentionActionAllow,
					Name:        "bar",
					Peer:        "peer1",
					Description: "peered service intention",
				},
				{
					Action:      structs.IntentionActionAllow,
					Name:        "bar",
					Description: "local service intention",
				},
			},
		},
	}
	idx := uint64(0)

	for _, input := range inputs {
		require.NoError(t, input.Normalize())
		require.NoError(t, input.Validate())
		idx++
		require.NoError(t, s.EnsureConfigEntry(idx, input))
	}

	t.Run("assert that we can get the peered intention", func(t *testing.T) {
		idx, entry, ixn, err := s.IntentionGetExact(nil, &structs.IntentionQueryExact{
			SourceNS:        "default",
			SourceName:      "bar",
			SourcePeer:      "peer1",
			DestinationNS:   "default",
			DestinationName: "foo",
		})

		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NotNil(t, ixn)
		require.Equal(t, "peer1", ixn.SourcePeer)
		require.Equal(t, "peered service intention", ixn.Description)
		require.Equal(t, uint64(1), idx)
	})

	t.Run("assert that we can get the local intention", func(t *testing.T) {
		idx, entry, ixn, err := s.IntentionGetExact(nil, &structs.IntentionQueryExact{
			SourceNS:        "default",
			SourceName:      "bar",
			DestinationNS:   "default",
			DestinationName: "foo",
		})

		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NotNil(t, ixn)
		require.Empty(t, ixn.SourcePeer)
		require.Equal(t, "local service intention", ixn.Description)
		require.Equal(t, uint64(1), idx)
	})
}

func TestStore_IntentionMatch_ConfigEntries(t *testing.T) {
	type testcase struct {
		name          string
		configEntries []structs.ConfigEntry
		query         structs.IntentionQueryMatch
		expect        []structs.Intentions
	}
	run := func(t *testing.T, tc testcase) {
		s := testConfigStateStore(t)
		idx := uint64(0)
		for _, conf := range tc.configEntries {
			require.NoError(t, conf.Normalize())
			require.NoError(t, conf.Validate())
			idx++
			require.NoError(t, s.EnsureConfigEntry(idx, conf))
		}

		_, matches, err := s.IntentionMatch(nil, &tc.query)
		require.NoError(t, err)

		// clear raft indexes for easier comparison
		for _, match := range matches {
			for _, ixn := range match {
				ixn.CreateIndex = 0
				ixn.ModifyIndex = 0
			}
		}
		require.Equal(t, tc.expect, matches)
	}
	tcs := []testcase{
		{
			name: "peered intention matched with destination query",
			configEntries: []structs.ConfigEntry{
				&structs.ServiceIntentionsConfigEntry{
					Kind: structs.ServiceIntentions,
					Name: "foo",
					Sources: []*structs.SourceIntention{
						{
							Action: structs.IntentionActionAllow,
							Name:   "example",
							Peer:   "bar",
						},
						{
							Action: structs.IntentionActionAllow,
							Name:   "*",
							Peer:   "baz",
						},
					},
				},
			},
			query: structs.IntentionQueryMatch{
				Type: structs.IntentionMatchDestination,
				Entries: []structs.IntentionMatchEntry{
					{
						Namespace: "default",
						Name:      "foo",
					},
				},
			},
			expect: []structs.Intentions{
				{
					{
						Action:               structs.IntentionActionAllow,
						SourceType:           structs.IntentionSourceConsul,
						DestinationPartition: acl.DefaultPartitionName,
						DestinationNS:        "default",
						DestinationName:      "foo",
						SourcePeer:           "bar",
						SourceNS:             "default",
						SourceName:           "example",
						SourcePartition:      "", // note that SourcePartition does not get normalized
						Precedence:           9,
					},
					{
						Action:               structs.IntentionActionAllow,
						SourceType:           structs.IntentionSourceConsul,
						DestinationPartition: acl.DefaultPartitionName,
						DestinationNS:        "default",
						DestinationName:      "foo",
						SourcePeer:           "baz",
						SourceNS:             "default",
						SourceName:           "*",
						SourcePartition:      "", // note that SourcePartition does not get normalized
						Precedence:           8,
					},
				},
			},
		},
		{
			// This behavior may change in the future but this test is in place
			// to ensure peered intentions cannot accidentally be queried by source
			name: "peered intention cannot be queried by source",
			configEntries: []structs.ConfigEntry{
				&structs.ServiceIntentionsConfigEntry{
					Kind: structs.ServiceIntentions,
					Name: "foo",
					Sources: []*structs.SourceIntention{
						{
							Action: structs.IntentionActionAllow,
							Name:   "example",
							Peer:   "bar",
						},
					},
				},
			},
			query: structs.IntentionQueryMatch{
				Type: structs.IntentionMatchSource,
				Entries: []structs.IntentionMatchEntry{
					{
						// We don't expose a Peer field
						Namespace: "default",
						Name:      "example",
					},
				},
			},
			expect: []structs.Intentions{nil},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

// Test the matrix of match logic.
//
// Note that this doesn't need to test the intention sort logic exhaustively
// since this is tested in their sort implementation in the structs.
// TODO(partitions): Update for partition matching
func TestStore_IntentionMatch_table(t *testing.T) {
	type testCase struct {
		Name     string
		Insert   [][]string   // List of intentions to insert
		Query    [][]string   // List of intentions to match
		Expected [][][]string // List of matches, where each match is a list of intentions
	}

	cases := []testCase{
		{
			"single exact name",
			[][]string{
				{"bar", "example"},
				{"baz", "example"}, // shouldn't match
				{"*", "example"},
			},
			[][]string{
				{"bar"},
			},
			[][][]string{
				{
					{"bar", "example"},
					{"*", "example"},
				},
			},
		},
		{
			"multiple exact name",
			[][]string{
				{"bar", "example"},
				{"baz", "example"}, // shouldn't match
				{"*", "example"},
			},
			[][]string{
				{"bar"},
				{"baz"},
			},
			[][][]string{
				{
					{"bar", "example"},
					{"*", "example"},
				},
				{
					{"baz", "example"},
					{"*", "example"},
				},
			},
		},

		{
			"single exact name with duplicate destinations",
			[][]string{
				// 2-tuple specifies src and destination to test duplicate destinations
				// with different sources. We flip them around to test in both
				// directions. The first pair are the ones searched on in both cases so
				// the duplicates need to be there.
				{"bar", "*"},
				{"*", "*"},
			},
			[][]string{
				{"bar"},
			},
			[][][]string{
				{
					{"bar", "*"},
					{"*", "*"},
				},
			},
		},
	}

	// testRunner implements the test for a single case, but can be
	// parameterized to run for both source and destination so we can
	// test both cases.
	testRunner := func(t *testing.T, s *Store, legacy bool, tc testCase, typ structs.IntentionMatchType) {
		lastIndex := uint64(0)
		if legacy {
			lastIndex = 1 // minor state machine implementation difference
		}

		// Insert the set
		var ixns []*structs.Intention
		for _, v := range tc.Insert {
			if len(v) != 2 {
				panic("invalid input")
			}
			ixn := &structs.Intention{
				ID:     testUUID(),
				Action: structs.IntentionActionAllow,
			}
			switch typ {
			case structs.IntentionMatchDestination:
				ixn.DestinationNS = "default"
				ixn.DestinationName = v[0]
				ixn.SourceNS = "default"
				ixn.SourceName = v[1]
			case structs.IntentionMatchSource:
				ixn.SourceNS = "default"
				ixn.SourceName = v[0]
				ixn.DestinationNS = "default"
				ixn.DestinationName = v[1]
			default:
				panic("unexpected")
			}
			ixns = append(ixns, ixn)
		}

		if legacy {
			for _, ixn := range ixns {
				lastIndex++
				require.NoError(t, s.LegacyIntentionSet(lastIndex, ixn))
			}
		} else {
			entries := structs.MigrateIntentions(ixns)
			for _, conf := range entries {
				require.NoError(t, conf.LegacyNormalize())
				require.NoError(t, conf.LegacyValidate())
				lastIndex++
				require.NoError(t, s.EnsureConfigEntry(lastIndex, conf))
			}
		}

		// Build the arguments
		args := &structs.IntentionQueryMatch{Type: typ}
		for _, q := range tc.Query {
			if len(q) != 1 {
				panic("wrong length")
			}
			args.Entries = append(args.Entries, structs.IntentionMatchEntry{
				Namespace: "default",
				Name:      q[0],
			})
		}

		// Match
		_, matches, err := s.IntentionMatch(nil, args)
		require.NoError(t, err)

		// Should have equal lengths
		require.Len(t, matches, len(tc.Expected))

		// Verify matches
		for i, expected := range tc.Expected {
			for _, exp := range expected {
				if len(exp) != 2 {
					panic("invalid input")
				}
			}
			var actual [][]string
			for _, ixn := range matches[i] {
				switch typ {
				case structs.IntentionMatchDestination:
					actual = append(actual, []string{
						ixn.DestinationName,
						ixn.SourceName,
					})
				case structs.IntentionMatchSource:
					actual = append(actual, []string{
						ixn.SourceName,
						ixn.DestinationName,
					})
				default:
					panic("unexpected")
				}
			}

			require.Equal(t, expected, actual)
		}
	}

	for _, tc := range cases {
		t.Run(tc.Name+" (destination)", func(t *testing.T) {
			testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
				testRunner(t, s, legacy, tc, structs.IntentionMatchDestination)
			})
		})

		t.Run(tc.Name+" (source)", func(t *testing.T) {
			testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
				testRunner(t, s, legacy, tc, structs.IntentionMatchSource)
			})
		})
	}
}

// Equivalent to TestStore_IntentionMatch_table but for IntentionMatchOne which
// matches a single service
// TODO(partitions): Update for partition matching
func TestStore_IntentionMatchOne_table(t *testing.T) {
	type testCase struct {
		Name     string
		Insert   [][]string   // List of intentions to insert
		Query    []string     // List of intentions to match
		Expected [][][]string // List of matches, where each match is a list of intentions
	}

	cases := []testCase{
		{
			"stress test the intention-source index on config entries",
			[][]string{
				{"foo", "bar"},
				{"foo", "baz"},
				{"foo", "zab"},
				{"oof", "bar"},
				{"oof", "baz"},
				{"oof", "zab"},
			},
			[]string{
				"foo",
				"oof",
			},
			[][][]string{
				{
					{"foo", "bar"},
					{"foo", "baz"},
					{"foo", "zab"},
				},
				{
					{"oof", "bar"},
					{"oof", "baz"},
					{"oof", "zab"},
				},
			},
		},
		{
			"single exact name",
			[][]string{
				{"bar", "example"},
				{"baz", "example"}, // shouldn't match
				{"*", "example"},
			},
			[]string{
				"bar",
			},
			[][][]string{
				{
					{"bar", "example"},
					{"*", "example"},
				},
			},
		},
		{
			"single exact name with duplicate destinations",
			[][]string{
				// 2-tuple specifies src and destination to test duplicate destinations
				// with different sources. We flip them around to test in both
				// directions. The first pair are the ones searched on in both cases so
				// the duplicates need to be there.
				{"bar", "*"},
				{"*", "*"},
			},
			[]string{
				"bar",
			},
			[][][]string{
				{
					{"bar", "*"},
					{"*", "*"},
				},
			},
		},
	}

	testRunner := func(t *testing.T, s *Store, legacy bool, tc testCase, typ structs.IntentionMatchType) {
		lastIndex := uint64(0)
		if legacy {
			lastIndex = 1 // minor state machine implementation difference
		}

		// Insert the set
		var ixns []*structs.Intention
		for _, v := range tc.Insert {
			if len(v) != 2 {
				panic("invalid input")
			}
			ixn := &structs.Intention{
				ID:     testUUID(),
				Action: structs.IntentionActionAllow,
			}
			switch typ {
			case structs.IntentionMatchSource:
				ixn.SourceNS = "default"
				ixn.SourceName = v[0]
				ixn.DestinationNS = "default"
				ixn.DestinationName = v[1]
			case structs.IntentionMatchDestination:
				ixn.DestinationNS = "default"
				ixn.DestinationName = v[0]
				ixn.SourceNS = "default"
				ixn.SourceName = v[1]
			default:
				panic("unexpected")
			}
			ixns = append(ixns, ixn)
		}

		if legacy {
			for _, ixn := range ixns {
				lastIndex++
				require.NoError(t, s.LegacyIntentionSet(lastIndex, ixn))
			}
		} else {
			entries := structs.MigrateIntentions(ixns)
			for _, conf := range entries {
				require.NoError(t, conf.LegacyNormalize())
				require.NoError(t, conf.LegacyValidate())
				lastIndex++
				require.NoError(t, s.EnsureConfigEntry(lastIndex, conf))
			}
		}

		if len(tc.Expected) != len(tc.Query) {
			panic("invalid input")
		}

		for i, query := range tc.Query {
			expected := tc.Expected[i]
			for _, exp := range expected {
				if len(exp) != 2 {
					panic("invalid input")
				}
			}

			t.Run("query: "+query, func(t *testing.T) {
				// Build the arguments and match
				entry := structs.IntentionMatchEntry{
					Namespace: "default",
					Name:      query,
				}
				_, matches, err := s.IntentionMatchOne(nil, entry, typ, structs.IntentionTargetService)
				require.NoError(t, err)

				// Verify matches
				var actual [][]string
				for _, ixn := range matches {
					switch typ {
					case structs.IntentionMatchDestination:
						actual = append(actual, []string{
							ixn.DestinationName,
							ixn.SourceName,
						})
					case structs.IntentionMatchSource:
						actual = append(actual, []string{
							ixn.SourceName,
							ixn.DestinationName,
						})
					}
				}
				require.Equal(t, expected, actual)
			})
		}
	}

	for _, tc := range cases {
		t.Run(tc.Name+" (destination)", func(t *testing.T) {
			testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
				testRunner(t, s, legacy, tc, structs.IntentionMatchDestination)
			})
		})

		t.Run(tc.Name+" (source)", func(t *testing.T) {
			testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
				testRunner(t, s, legacy, tc, structs.IntentionMatchSource)
			})
		})
	}
}

func TestStore_IntentionMatch_WatchesDuringUpgrade(t *testing.T) {
	s := testStateStore(t)

	args := structs.IntentionQueryMatch{
		Type: structs.IntentionMatchDestination,
		Entries: []structs.IntentionMatchEntry{
			{Namespace: "default", Name: "api"},
		},
	}

	// Start with an empty, un-upgraded database and do a watch.

	ws := memdb.NewWatchSet()
	_, matches, err := s.IntentionMatch(ws, &args)
	require.NoError(t, err)
	require.Len(t, matches, 1)    // one request gets one response
	require.Len(t, matches[0], 0) // but no intentions

	disableLegacyIntentions(s)
	conf := &structs.ServiceIntentionsConfigEntry{
		Kind: structs.ServiceIntentions,
		Name: "api",
		Sources: []*structs.SourceIntention{
			{Name: "web", Action: structs.IntentionActionAllow},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(1, conf))

	require.True(t, watchFired(ws))
}

func TestStore_LegacyIntention_Snapshot_Restore(t *testing.T) {
	// note: irrelevant test for config entries variant
	s := testStateStore(t)

	// Create some intentions.
	ixns := structs.Intentions{
		&structs.Intention{
			DestinationName: "foo",
		},
		&structs.Intention{
			DestinationName: "bar",
		},
		&structs.Intention{
			DestinationName: "baz",
		},
	}

	// Force the sort order of the UUIDs before we create them so the
	// order is deterministic.
	id := testUUID()
	ixns[0].ID = "a" + id[1:]
	ixns[1].ID = "b" + id[1:]
	ixns[2].ID = "c" + id[1:]

	// Now create
	for i, ixn := range ixns {
		require.NoError(t, s.LegacyIntentionSet(uint64(4+i), ixn))
	}

	// Snapshot the queries.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	require.NoError(t, s.LegacyIntentionDelete(7, ixns[0].ID))

	// Verify the snapshot.
	require.Equal(t, snap.LastIndex(), uint64(6))

	// Expect them sorted in insertion order
	expected := structs.Intentions{
		&structs.Intention{
			ID:              ixns[0].ID,
			DestinationName: "foo",
			Meta:            map[string]string{},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 4,
				ModifyIndex: 4,
			},
		},
		&structs.Intention{
			ID:              ixns[1].ID,
			DestinationName: "bar",
			Meta:            map[string]string{},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 5,
				ModifyIndex: 5,
			},
		},
		&structs.Intention{
			ID:              ixns[2].ID,
			DestinationName: "baz",
			Meta:            map[string]string{},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 6,
				ModifyIndex: 6,
			},
		},
	}
	for i := range expected {
		//nolint:staticcheck
		expected[i].UpdatePrecedence() // to match what is returned...
	}
	dump, err := snap.LegacyIntentions()
	require.NoError(t, err)
	require.Equal(t, expected, dump)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, ixn := range dump {
			require.NoError(t, restore.LegacyIntention(ixn))
		}
		restore.Commit()

		// Read the restored values back out and verify that they match. Note that
		// Intentions are returned precedence sorted unlike the snapshot so we need
		// to rearrange the expected slice some.
		expected[0], expected[1], expected[2] = expected[1], expected[2], expected[0]
		entMeta := structs.WildcardEnterpriseMetaInDefaultPartition()
		idx, actual, fromConfig, err := s.Intentions(nil, entMeta)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(6))
		require.False(t, fromConfig)
		require.Equal(t, expected, actual)
	}()
}

// Note: This test does not have an equivalent with legacy intentions as an input.
// That's because the config vs legacy split is handled by store.IntentionMatch
// which has its own tests
func TestStore_IntentionDecision(t *testing.T) {
	// web to redis allowed and with permissions
	// api to redis denied and without perms (so redis has multiple matches as destination)
	// api to web without permissions and with meta
	entries := []structs.ConfigEntry{
		&structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": "http",
			},
		},
		&structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "redis",
			Sources: []*structs.SourceIntention{
				{
					Name: "web",
					Permissions: []*structs.IntentionPermission{
						{
							Action: structs.IntentionActionAllow,
							HTTP: &structs.IntentionHTTPPermission{
								Methods: []string{"GET"},
							},
						},
					},
				},
				{
					Name:   "api",
					Action: structs.IntentionActionDeny,
				},
			},
		},
		&structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "web",
			Meta: map[string]string{structs.MetaExternalSource: "nomad"},
			Sources: []*structs.SourceIntention{
				{
					Name:   "api",
					Action: structs.IntentionActionAllow,
				},
			},
		},
		&structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "mysql",
			Sources: []*structs.SourceIntention{
				{
					Name:   "*",
					Action: structs.IntentionActionAllow,
				},
			},
		},
	}

	s := testConfigStateStore(t)
	for _, entry := range entries {
		require.NoError(t, s.EnsureConfigEntry(1, entry))
	}

	tt := []struct {
		name             string
		src              string
		dst              string
		matchType        structs.IntentionMatchType
		defaultDecision  acl.EnforcementDecision
		allowPermissions bool
		expect           structs.IntentionDecisionSummary
	}{
		{
			name:            "no matching intention and default deny",
			src:             "does-not-exist",
			dst:             "ditto",
			matchType:       structs.IntentionMatchDestination,
			defaultDecision: acl.Deny,
			expect: structs.IntentionDecisionSummary{
				Allowed:      false,
				DefaultAllow: false,
			},
		},
		{
			name:            "no matching intention and default allow",
			src:             "does-not-exist",
			dst:             "ditto",
			matchType:       structs.IntentionMatchDestination,
			defaultDecision: acl.Allow,
			expect: structs.IntentionDecisionSummary{
				Allowed:      true,
				DefaultAllow: true,
			},
		},
		{
			name:      "denied with permissions",
			src:       "web",
			dst:       "redis",
			matchType: structs.IntentionMatchDestination,
			expect: structs.IntentionDecisionSummary{
				Allowed:        false,
				HasPermissions: true,
				HasExact:       true,
			},
		},
		{
			name:             "allowed with permissions",
			src:              "web",
			dst:              "redis",
			allowPermissions: true,
			matchType:        structs.IntentionMatchDestination,
			expect: structs.IntentionDecisionSummary{
				Allowed:        true,
				HasPermissions: true,
				HasExact:       true,
			},
		},
		{
			name:      "denied without permissions",
			src:       "api",
			dst:       "redis",
			matchType: structs.IntentionMatchDestination,
			expect: structs.IntentionDecisionSummary{
				Allowed:        false,
				HasPermissions: false,
				HasExact:       true,
			},
		},
		{
			name:      "allowed from external source",
			src:       "api",
			dst:       "web",
			matchType: structs.IntentionMatchDestination,
			expect: structs.IntentionDecisionSummary{
				Allowed:        true,
				HasPermissions: false,
				ExternalSource: "nomad",
				HasExact:       true,
			},
		},
		{
			name:      "allowed by source wildcard not exact",
			src:       "anything",
			dst:       "mysql",
			matchType: structs.IntentionMatchDestination,
			expect: structs.IntentionDecisionSummary{
				Allowed:        true,
				HasPermissions: false,
				HasExact:       false,
			},
		},
		{
			name:      "allowed by matching on source",
			src:       "web",
			dst:       "api",
			matchType: structs.IntentionMatchSource,
			expect: structs.IntentionDecisionSummary{
				Allowed:        true,
				HasPermissions: false,
				HasExact:       false,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			entry := structs.IntentionMatchEntry{
				Namespace: structs.IntentionDefaultNamespace,
				Partition: acl.DefaultPartitionName,
				Name:      tc.src,
			}
			_, intentions, err := s.IntentionMatchOne(nil, entry, structs.IntentionMatchSource, structs.IntentionTargetService)
			if err != nil {
				require.NoError(t, err)
			}

			opts := IntentionDecisionOpts{
				Target:           tc.dst,
				Namespace:        structs.IntentionDefaultNamespace,
				Partition:        acl.DefaultPartitionName,
				Intentions:       intentions,
				MatchType:        tc.matchType,
				DefaultDecision:  tc.defaultDecision,
				AllowPermissions: tc.allowPermissions,
			}
			decision, err := s.IntentionDecision(opts)
			require.NoError(t, err)
			require.Equal(t, tc.expect, decision)
		})
	}
}

func disableLegacyIntentions(s *Store) error {
	return s.SystemMetadataSet(1, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataIntentionFormatKey,
		Value: structs.SystemMetadataIntentionFormatConfigValue,
	})
}

func testConfigStateStore(t *testing.T) *Store {
	s := testStateStore(t)
	s.SystemMetadataSet(5, &structs.SystemMetadataEntry{Key: structs.SystemMetadataVirtualIPsEnabled, Value: "true"})
	disableLegacyIntentions(s)
	return s
}

func TestStore_IntentionTopology(t *testing.T) {
	node := structs.Node{
		Node:    "foo",
		Address: "127.0.0.1",
	}
	services := []structs.NodeService{
		{
			ID:             structs.ConsulServiceID,
			Service:        structs.ConsulServiceName,
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			ID:             "api-1",
			Service:        "api",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			ID:             "mysql-1",
			Service:        "mysql",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			ID:             "web-1",
			Service:        "web",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			Kind:           structs.ServiceKindConnectProxy,
			ID:             "web-proxy-1",
			Service:        "web-proxy",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			Kind:           structs.ServiceKindTerminatingGateway,
			ID:             "terminating-gateway-1",
			Service:        "terminating-gateway",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			Kind:           structs.ServiceKindIngressGateway,
			ID:             "ingress-gateway-1",
			Service:        "ingress-gateway",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			Kind:           structs.ServiceKindMeshGateway,
			ID:             "mesh-gateway-1",
			Service:        "mesh-gateway",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}

	type expect struct {
		idx      uint64
		services structs.ServiceList
	}
	tests := []struct {
		name            string
		defaultDecision acl.EnforcementDecision
		intentions      []structs.ServiceIntentionsConfigEntry
		discoveryChains []structs.ConfigEntry
		target          structs.ServiceName
		downstreams     bool
		expect          expect
	}{
		{
			name:            "(upstream) acl allow all but intentions deny one",
			defaultDecision: acl.Allow,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "api",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionDeny,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 10,
				services: structs.ServiceList{
					{
						Name:           "mysql",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "(upstream) acl allow includes virtual service",
			defaultDecision: acl.Allow,
			discoveryChains: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "backend",
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 10,
				services: structs.ServiceList{
					{
						Name:           "api",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "backend",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "mysql",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "(upstream) acl deny all intentions allow virtual service",
			defaultDecision: acl.Deny,
			discoveryChains: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "backend",
				},
			},
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "backend",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 11,
				services: structs.ServiceList{
					{
						Name:           "backend",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "(upstream) acl deny all intentions allow one",
			defaultDecision: acl.Deny,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "api",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 10,
				services: structs.ServiceList{
					{
						Name:           "api",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "(downstream) acl allow all but intentions deny one",
			defaultDecision: acl.Allow,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "api",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionDeny,
						},
					},
				},
			},
			target:      structs.NewServiceName("api", nil),
			downstreams: true,
			expect: expect{
				idx: 10,
				services: structs.ServiceList{
					{
						Name:           "ingress-gateway",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "mysql",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "(downstream) acl deny all intentions allow one",
			defaultDecision: acl.Deny,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "api",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			target:      structs.NewServiceName("api", nil),
			downstreams: true,
			expect: expect{
				idx: 10,
				services: structs.ServiceList{
					{
						Name:           "web",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "acl deny but intention allow all overrides it",
			defaultDecision: acl.Deny,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "*",
					Sources: []*structs.SourceIntention{
						{
							Name:   "*",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 10,
				services: structs.ServiceList{
					{
						Name:           "api",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "mysql",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "acl allow but intention deny all overrides it",
			defaultDecision: acl.Allow,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "*",
					Sources: []*structs.SourceIntention{
						{
							Name:   "*",
							Action: structs.IntentionActionDeny,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx:      10,
				services: structs.ServiceList{},
			},
		},
		{
			name:            "acl deny but intention allow all overrides it",
			defaultDecision: acl.Deny,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "*",
					Sources: []*structs.SourceIntention{
						{
							Name:   "*",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 10,
				services: structs.ServiceList{
					{
						Name:           "api",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "mysql",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testConfigStateStore(t)

			var idx uint64 = 1
			require.NoError(t, s.EnsureNode(idx, &node))
			idx++

			for _, svc := range services {
				require.NoError(t, s.EnsureService(idx, "foo", &svc))
				idx++
			}
			for _, ixn := range tt.intentions {
				require.NoError(t, s.EnsureConfigEntry(idx, &ixn))
				idx++
			}
			for _, entry := range tt.discoveryChains {
				require.NoError(t, s.EnsureConfigEntry(idx, entry))
				idx++
			}

			idx, got, err := s.IntentionTopology(nil, tt.target, tt.downstreams, tt.defaultDecision, structs.IntentionTargetService)
			require.NoError(t, err)
			require.Equal(t, tt.expect.idx, idx)

			// ServiceList is from a map, so it is not deterministically sorted
			sort.Slice(got, func(i, j int) bool {
				return got[i].String() < got[j].String()
			})
			require.Equal(t, tt.expect.services, got)
		})
	}
}

func TestStore_IntentionTopology_Destination(t *testing.T) {
	node := structs.Node{
		Node:    "foo",
		Address: "127.0.0.1",
	}

	services := []structs.NodeService{
		{
			ID:             structs.ConsulServiceID,
			Service:        structs.ConsulServiceName,
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			ID:             "web-1",
			Service:        "web",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			ID:             "mysql-1",
			Service:        "mysql",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}
	destinations := []structs.ServiceConfigEntry{
		{
			Name:           "api.test.com",
			Destination:    &structs.DestinationConfig{},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			Name:           "kafka.store.org",
			Destination:    &structs.DestinationConfig{},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}

	type expect struct {
		idx      uint64
		services structs.ServiceList
	}
	tests := []struct {
		name            string
		defaultDecision acl.EnforcementDecision
		intentions      []structs.ServiceIntentionsConfigEntry
		target          structs.ServiceName
		downstreams     bool
		expect          expect
	}{
		{
			name:            "(upstream) acl allow all but intentions deny one, destination target",
			defaultDecision: acl.Allow,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "api.test.com",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionDeny,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 7,
				services: structs.ServiceList{
					{
						Name:           "kafka.store.org",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "(upstream) acl deny all intentions allow one, destination target",
			defaultDecision: acl.Deny,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "kafka.store.org",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 7,
				services: structs.ServiceList{
					{
						Name:           "kafka.store.org",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
		{
			name:            "(upstream) acl deny all check only destinations show, service target",
			defaultDecision: acl.Deny,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "api",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx:      7,
				services: structs.ServiceList{},
			},
		},
		{
			name:            "(upstream) acl allow all check only destinations show, service target",
			defaultDecision: acl.Allow,
			intentions: []structs.ServiceIntentionsConfigEntry{
				{
					Kind: structs.ServiceIntentions,
					Name: "api",
					Sources: []*structs.SourceIntention{
						{
							Name:   "web",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			target:      structs.NewServiceName("web", nil),
			downstreams: false,
			expect: expect{
				idx: 7,
				services: structs.ServiceList{
					{
						Name:           "api.test.com",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "kafka.store.org",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testConfigStateStore(t)

			var idx uint64 = 1
			require.NoError(t, s.EnsureNode(idx, &node))
			idx++

			for _, svc := range services {
				require.NoError(t, s.EnsureService(idx, "foo", &svc))
				idx++
			}
			for _, d := range destinations {
				require.NoError(t, s.EnsureConfigEntry(idx, &d))
				idx++
			}
			for _, ixn := range tt.intentions {
				require.NoError(t, s.EnsureConfigEntry(idx, &ixn))
				idx++
			}

			idx, got, err := s.IntentionTopology(nil, tt.target, tt.downstreams, tt.defaultDecision, structs.IntentionTargetDestination)
			require.NoError(t, err)
			require.Equal(t, tt.expect.idx, idx)

			// ServiceList is from a map, so it is not deterministically sorted
			sort.Slice(got, func(i, j int) bool {
				return got[i].String() < got[j].String()
			})
			require.Equal(t, tt.expect.services, got)
		})
	}
}

func TestStore_IntentionTopology_Watches(t *testing.T) {
	s := testConfigStateStore(t)
	s.SystemMetadataSet(10, &structs.SystemMetadataEntry{Key: structs.SystemMetadataVirtualIPsEnabled, Value: "true"})

	var i uint64 = 1
	require.NoError(t, s.EnsureNode(i, &structs.Node{
		Node:    "foo",
		Address: "127.0.0.1",
	}))
	i++

	target := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())

	ws := memdb.NewWatchSet()
	index, got, err := s.IntentionTopology(ws, target, false, acl.Deny, structs.IntentionTargetService)
	require.NoError(t, err)
	require.Equal(t, uint64(0), index)
	require.Empty(t, got)

	// Watch should fire after adding a relevant config entry
	require.NoError(t, s.EnsureConfigEntry(i, &structs.ServiceIntentionsConfigEntry{
		Kind: structs.ServiceIntentions,
		Name: "api",
		Sources: []*structs.SourceIntention{
			{
				Name:   "web",
				Action: structs.IntentionActionAllow,
			},
		},
	}))
	i++

	require.True(t, watchFired(ws))

	// Reset the WatchSet
	ws = memdb.NewWatchSet()
	index, got, err = s.IntentionTopology(ws, target, false, acl.Deny, structs.IntentionTargetService)
	require.NoError(t, err)
	require.Equal(t, uint64(2), index)
	// Because API is a virtual service, it is included in this output.
	require.Equal(t, structs.ServiceList{structs.NewServiceName("api", nil)}, got)

	// Watch should not fire after unrelated intention changes
	require.NoError(t, s.EnsureConfigEntry(i, &structs.ServiceIntentionsConfigEntry{
		Kind: structs.ServiceIntentions,
		Name: "another service",
		Sources: []*structs.SourceIntention{
			{
				Name:   "any other service",
				Action: structs.IntentionActionAllow,
			},
		},
	}))
	i++
	// TODO(freddy) Why is this firing?
	// require.False(t, watchFired(ws))

	// Result should not have changed
	index, got, err = s.IntentionTopology(ws, target, false, acl.Deny, structs.IntentionTargetService)
	require.NoError(t, err)
	require.Equal(t, uint64(3), index)
	require.Equal(t, structs.ServiceList{structs.NewServiceName("api", nil)}, got)

	// Watch should fire after service list changes
	require.NoError(t, s.EnsureService(i, "foo", &structs.NodeService{
		ID:             "api-1",
		Service:        "api",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}))

	require.True(t, watchFired(ws))

	// Reset the WatchSet
	index, got, err = s.IntentionTopology(nil, target, false, acl.Deny, structs.IntentionTargetService)
	require.NoError(t, err)
	require.Equal(t, uint64(4), index)

	expect := structs.ServiceList{
		{
			Name:           "api",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}
	require.Equal(t, expect, got)
}
