package state

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert := assert.New(t)

		// Querying with no results returns nil.
		ws := memdb.NewWatchSet()
		idx, _, res, err := s.IntentionGet(ws, testUUID())
		assert.Equal(uint64(1), idx)
		assert.Nil(res)
		assert.Nil(err)
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
			}

			// Inserting a with empty ID is disallowed.
			lastIndex++
			require.NoError(t, s.LegacyIntentionSet(lastIndex, legacyIxn))

			// Make sure the right index got updated.
			require.Equal(t, lastIndex, s.maxIndex(intentionsTableName))
			require.Equal(t, uint64(0), s.maxIndex(configTableName))

			expected = &structs.Intention{
				ID:              legacyIxn.ID,
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "web",
				Meta:            map[string]string{},
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
						LegacyID:   srcID,
						Name:       "*",
						Action:     structs.IntentionActionAllow,
						LegacyMeta: map[string]string{},
					},
				},
			}

			lastIndex++
			require.NoError(t, configEntry.LegacyNormalize())
			require.NoError(t, configEntry.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(lastIndex, configEntry.Clone(), nil))

			// Make sure the config entry index got updated instead of the old intentions one
			require.Equal(t, lastIndex, s.maxIndex(configTableName))
			require.Equal(t, uint64(0), s.maxIndex(intentionsTableName))

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
			require.Equal(t, lastIndex, s.maxIndex(intentionsTableName))
			require.Equal(t, uint64(0), s.maxIndex(configTableName))

			expected.SourceNS = legacyIxn.SourceNS
			expected.Action = structs.IntentionActionDeny
			expected.ModifyIndex = lastIndex

		} else {
			// Change a value and test updating
			configEntry.Sources[0].Description = "test-desc1"
			lastIndex++
			require.NoError(t, configEntry.LegacyNormalize())
			require.NoError(t, configEntry.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(lastIndex, configEntry.Clone(), nil))

			// Change a value that isn't in the unique 4 tuple and check we don't
			// incorrectly consider this a duplicate when updating.
			configEntry.Sources[0].Action = structs.IntentionActionDeny
			lastIndex++
			require.NoError(t, configEntry.LegacyNormalize())
			require.NoError(t, configEntry.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(lastIndex, configEntry.Clone(), nil))

			// Make sure the config entry index got updated instead of the old intentions one
			require.Equal(t, lastIndex, s.maxIndex(configTableName))
			require.Equal(t, uint64(0), s.maxIndex(intentionsTableName))

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
			require.Equal(t, lastIndex-1, s.maxIndex(intentionsTableName))
			require.Equal(t, uint64(0), s.maxIndex(configTableName))
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
	require.Equal(t, s.maxIndex(intentionsTableName), uint64(0))
	require.Equal(t, uint64(0), s.maxIndex(configTableName))

	require.False(t, watchFired(ws), "watch fired")
}

func TestStore_IntentionSet_updateCreatedAt(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		// Build a valid intention
		var (
			id         = testUUID()
			createTime time.Time
		)

		if legacy {
			ixn := structs.Intention{
				ID:        id,
				CreatedAt: time.Now().UTC(),
			}

			// Insert
			require.NoError(t, s.LegacyIntentionSet(1, &ixn))

			createTime = ixn.CreatedAt

			// Change a value and test updating
			ixnUpdate := ixn
			ixnUpdate.CreatedAt = createTime.Add(10 * time.Second)
			require.NoError(t, s.LegacyIntentionSet(2, &ixnUpdate))

			id = ixn.ID

		} else {
			conf := &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "web",
				Sources: []*structs.SourceIntention{
					{
						LegacyID:   id,
						Name:       "*",
						Action:     structs.IntentionActionAllow,
						LegacyMeta: map[string]string{},
					},
				},
			}

			require.NoError(t, conf.LegacyNormalize())
			require.NoError(t, conf.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(1, conf.Clone(), nil))

			createTime = *conf.Sources[0].LegacyCreateTime
		}

		// Read it back and verify
		_, _, actual, err := s.IntentionGet(nil, id)
		require.NoError(t, err)
		require.NotNil(t, actual)
		require.Equal(t, createTime, actual.CreatedAt)
	})
}

func TestStore_IntentionSet_metaNil(t *testing.T) {
	testBothIntentionFormats(t, func(t *testing.T, s *Store, legacy bool) {
		id := testUUID()
		if legacy {
			// Build a valid intention
			ixn := &structs.Intention{
				ID: id,
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
						LegacyID: id,
						Name:     "*",
						Action:   structs.IntentionActionAllow,
					},
				},
			}

			// Insert
			require.NoError(t, conf.LegacyNormalize())
			require.NoError(t, conf.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(1, conf.Clone(), nil))
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
				ID:   id,
				Meta: expectMeta,
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
						LegacyID:   id,
						Name:       "*",
						Action:     structs.IntentionActionAllow,
						LegacyMeta: expectMeta,
					},
				},
			}

			// Insert
			require.NoError(t, conf.LegacyNormalize())
			require.NoError(t, conf.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(1, conf.Clone(), nil))
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
				ID: id,
			}
			lastIndex++
			require.NoError(t, s.LegacyIntentionSet(lastIndex, ixn))

			// Make sure the index got updated.
			require.Equal(t, s.maxIndex(intentionsTableName), lastIndex)
			require.Equal(t, uint64(0), s.maxIndex(configTableName))
		} else {
			conf := &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "web",
				Sources: []*structs.SourceIntention{
					{
						LegacyID: id,
						Name:     "*",
						Action:   structs.IntentionActionAllow,
					},
				},
			}

			// Insert
			require.NoError(t, conf.LegacyNormalize())
			require.NoError(t, conf.LegacyValidate())
			require.NoError(t, s.EnsureConfigEntry(1, conf.Clone(), nil))

			// Make sure the index got updated.
			require.Equal(t, s.maxIndex(configTableName), lastIndex)
			require.Equal(t, uint64(0), s.maxIndex(intentionsTableName))
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
			require.Equal(t, s.maxIndex(intentionsTableName), lastIndex)
			require.Equal(t, uint64(0), s.maxIndex(configTableName))
		} else {
			lastIndex++
			require.NoError(t, s.DeleteConfigEntry(lastIndex, structs.ServiceIntentions, "web", nil))

			// Make sure the index got updated.
			require.Equal(t, s.maxIndex(configTableName), lastIndex)
			require.Equal(t, uint64(0), s.maxIndex(intentionsTableName))
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

		entMeta := structs.WildcardEnterpriseMeta()

		// Querying with no results returns nil.
		ws := memdb.NewWatchSet()
		idx, res, fromConfig, err := s.Intentions(ws, entMeta)
		require.NoError(t, err)
		require.Equal(t, !legacy, fromConfig)
		require.Nil(t, res)
		require.Equal(t, lastIndex, idx)

		testIntention := func(src, dst string) *structs.Intention {
			return &structs.Intention{
				ID:              testUUID(),
				SourceNS:        "default",
				SourceName:      src,
				DestinationNS:   "default",
				DestinationName: dst,
				SourceType:      structs.IntentionSourceConsul,
				Action:          structs.IntentionActionAllow,
				Meta:            map[string]string{},
			}
		}

		testConfigEntry := func(dst string, srcs ...string) *structs.ServiceIntentionsConfigEntry {
			conf := &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: dst,
			}
			id := testUUID()
			for _, src := range srcs {
				conf.Sources = append(conf.Sources, &structs.SourceIntention{
					LegacyID: id,
					Name:     src,
					Action:   structs.IntentionActionAllow,
				})
			}
			return conf
		}

		cmpIntention := func(ixn *structs.Intention, id string) *structs.Intention {
			ixn.ID = id
			//nolint:staticcheck
			ixn.UpdatePrecedence()
			return ixn
		}

		clearIrrelevantFields := func(ixns []*structs.Intention) {
			// Clear fields irrelevant for comparison.
			for _, ixn := range ixns {
				ixn.Hash = nil
				ixn.CreateIndex = 0
				ixn.ModifyIndex = 0
				ixn.CreatedAt = time.Time{}
				ixn.UpdatedAt = time.Time{}
			}
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
				require.NoError(t, s.EnsureConfigEntry(lastIndex, conf, nil))
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

		clearIrrelevantFields(actual)
		require.Equal(t, expected, actual)
	})
}

// Test the matrix of match logic.
//
// Note that this doesn't need to test the intention sort logic exhaustively
// since this is tested in their sort implementation in the structs.
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
				require.NoError(t, s.EnsureConfigEntry(lastIndex, conf, &conf.EnterpriseMeta))
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
				require.NoError(t, s.EnsureConfigEntry(lastIndex, conf, &conf.EnterpriseMeta))
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
				_, matches, err := s.IntentionMatchOne(nil, entry, typ)
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
		entMeta := structs.WildcardEnterpriseMeta()
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
	}

	s := testConfigStateStore(t)
	for _, entry := range entries {
		require.NoError(t, s.EnsureConfigEntry(1, entry, nil))
	}

	tt := []struct {
		name            string
		src             string
		dst             string
		defaultDecision acl.EnforcementDecision
		expect          structs.IntentionDecisionSummary
	}{
		{
			name:            "no matching intention and default deny",
			src:             "does-not-exist",
			dst:             "ditto",
			defaultDecision: acl.Deny,
			expect:          structs.IntentionDecisionSummary{Allowed: false},
		},
		{
			name:            "no matching intention and default allow",
			src:             "does-not-exist",
			dst:             "ditto",
			defaultDecision: acl.Allow,
			expect:          structs.IntentionDecisionSummary{Allowed: true},
		},
		{
			name: "denied with permissions",
			src:  "web",
			dst:  "redis",
			expect: structs.IntentionDecisionSummary{
				Allowed:        false,
				HasPermissions: true,
			},
		},
		{
			name: "denied without permissions",
			src:  "api",
			dst:  "redis",
			expect: structs.IntentionDecisionSummary{
				Allowed:        false,
				HasPermissions: false,
			},
		},
		{
			name: "allowed from external source",
			src:  "api",
			dst:  "web",
			expect: structs.IntentionDecisionSummary{
				Allowed:        true,
				HasPermissions: false,
				ExternalSource: "nomad",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			uri := connect.SpiffeIDService{
				Service:   tc.src,
				Namespace: structs.IntentionDefaultNamespace,
			}
			decision, err := s.IntentionDecision(&uri, tc.dst, structs.IntentionDefaultNamespace, tc.defaultDecision)
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
	disableLegacyIntentions(s)
	return s
}
