package state

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_IntentionGet_none(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Querying with no results returns nil.
	ws := memdb.NewWatchSet()
	idx, res, err := s.IntentionGet(ws, testUUID())
	assert.Equal(uint64(1), idx)
	assert.Nil(res)
	assert.Nil(err)
}

func TestStore_IntentionSetGet_basic(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Call Get to populate the watch set
	ws := memdb.NewWatchSet()
	_, _, err := s.IntentionGet(ws, testUUID())
	assert.Nil(err)

	// Build a valid intention
	ixn := &structs.Intention{
		ID:              testUUID(),
		SourceNS:        "default",
		SourceName:      "*",
		DestinationNS:   "default",
		DestinationName: "web",
		Meta:            map[string]string{},
	}

	// Inserting a with empty ID is disallowed.
	assert.NoError(s.IntentionSet(1, ixn))

	// Make sure the index got updated.
	assert.Equal(uint64(1), s.maxIndex(intentionsTableName))
	assert.True(watchFired(ws), "watch fired")

	// Read it back out and verify it.
	expected := &structs.Intention{
		ID:              ixn.ID,
		SourceNS:        "default",
		SourceName:      "*",
		DestinationNS:   "default",
		DestinationName: "web",
		Meta:            map[string]string{},
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}
	expected.UpdatePrecedence()

	ws = memdb.NewWatchSet()
	idx, actual, err := s.IntentionGet(ws, ixn.ID)
	assert.NoError(err)
	assert.Equal(expected.CreateIndex, idx)
	assert.Equal(expected, actual)

	// Change a value and test updating
	ixn.SourceNS = "foo"
	assert.NoError(s.IntentionSet(2, ixn))

	// Change a value that isn't in the unique 4 tuple and check we don't
	// incorrectly consider this a duplicate when updating.
	ixn.Action = structs.IntentionActionDeny
	assert.NoError(s.IntentionSet(2, ixn))

	// Make sure the index got updated.
	assert.Equal(uint64(2), s.maxIndex(intentionsTableName))
	assert.True(watchFired(ws), "watch fired")

	// Read it back and verify the data was updated
	expected.SourceNS = ixn.SourceNS
	expected.Action = structs.IntentionActionDeny
	expected.ModifyIndex = 2
	ws = memdb.NewWatchSet()
	idx, actual, err = s.IntentionGet(ws, ixn.ID)
	assert.NoError(err)
	assert.Equal(expected.ModifyIndex, idx)
	assert.Equal(expected, actual)

	// Attempt to insert another intention with duplicate 4-tuple
	ixn = &structs.Intention{
		ID:              testUUID(),
		SourceNS:        "default",
		SourceName:      "*",
		DestinationNS:   "default",
		DestinationName: "web",
		Meta:            map[string]string{},
	}

	// Duplicate 4-tuple should cause an error
	ws = memdb.NewWatchSet()
	assert.Error(s.IntentionSet(3, ixn))

	// Make sure the index did NOT get updated.
	assert.Equal(uint64(2), s.maxIndex(intentionsTableName))
	assert.False(watchFired(ws), "watch not fired")
}

func TestStore_IntentionSet_emptyId(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	ws := memdb.NewWatchSet()
	_, _, err := s.IntentionGet(ws, testUUID())
	assert.NoError(err)

	// Inserting a with empty ID is disallowed.
	err = s.IntentionSet(1, &structs.Intention{})
	assert.Error(err)
	assert.Contains(err.Error(), ErrMissingIntentionID.Error())

	// Index is not updated if nothing is saved.
	assert.Equal(s.maxIndex(intentionsTableName), uint64(0))
	assert.False(watchFired(ws), "watch fired")
}

func TestStore_IntentionSet_updateCreatedAt(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Build a valid intention
	now := time.Now()
	ixn := structs.Intention{
		ID:        testUUID(),
		CreatedAt: now,
	}

	// Insert
	assert.NoError(s.IntentionSet(1, &ixn))

	// Change a value and test updating
	ixnUpdate := ixn
	ixnUpdate.CreatedAt = now.Add(10 * time.Second)
	assert.NoError(s.IntentionSet(2, &ixnUpdate))

	// Read it back and verify
	_, actual, err := s.IntentionGet(nil, ixn.ID)
	assert.NoError(err)
	assert.Equal(now, actual.CreatedAt)
}

func TestStore_IntentionSet_metaNil(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Build a valid intention
	ixn := structs.Intention{
		ID: testUUID(),
	}

	// Insert
	assert.NoError(s.IntentionSet(1, &ixn))

	// Read it back and verify
	_, actual, err := s.IntentionGet(nil, ixn.ID)
	assert.NoError(err)
	assert.NotNil(actual.Meta)
}

func TestStore_IntentionSet_metaSet(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Build a valid intention
	ixn := structs.Intention{
		ID:   testUUID(),
		Meta: map[string]string{"foo": "bar"},
	}

	// Insert
	assert.NoError(s.IntentionSet(1, &ixn))

	// Read it back and verify
	_, actual, err := s.IntentionGet(nil, ixn.ID)
	assert.NoError(err)
	assert.Equal(ixn.Meta, actual.Meta)
}

func TestStore_IntentionDelete(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Call Get to populate the watch set
	ws := memdb.NewWatchSet()
	_, _, err := s.IntentionGet(ws, testUUID())
	assert.NoError(err)

	// Create
	ixn := &structs.Intention{ID: testUUID()}
	assert.NoError(s.IntentionSet(1, ixn))

	// Make sure the index got updated.
	assert.Equal(s.maxIndex(intentionsTableName), uint64(1))
	assert.True(watchFired(ws), "watch fired")

	// Delete
	assert.NoError(s.IntentionDelete(2, ixn.ID))

	// Make sure the index got updated.
	assert.Equal(s.maxIndex(intentionsTableName), uint64(2))
	assert.True(watchFired(ws), "watch fired")

	// Sanity check to make sure it's not there.
	idx, actual, err := s.IntentionGet(nil, ixn.ID)
	assert.NoError(err)
	assert.Equal(idx, uint64(2))
	assert.Nil(actual)
}

func TestStore_IntentionsList(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Querying with no results returns nil.
	ws := memdb.NewWatchSet()
	idx, res, err := s.Intentions(ws)
	assert.NoError(err)
	assert.Nil(res)
	assert.Equal(uint64(1), idx)

	// Create some intentions
	ixns := structs.Intentions{
		&structs.Intention{
			ID:   testUUID(),
			Meta: map[string]string{},
		},
		&structs.Intention{
			ID:   testUUID(),
			Meta: map[string]string{},
		},
	}

	// Force deterministic sort order
	ixns[0].ID = "a" + ixns[0].ID[1:]
	ixns[1].ID = "b" + ixns[1].ID[1:]

	// Create
	for i, ixn := range ixns {
		assert.NoError(s.IntentionSet(uint64(1+i), ixn))
	}
	assert.True(watchFired(ws), "watch fired")

	// Read it back and verify.
	expected := structs.Intentions{
		&structs.Intention{
			ID:   ixns[0].ID,
			Meta: map[string]string{},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		&structs.Intention{
			ID:   ixns[1].ID,
			Meta: map[string]string{},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		},
	}
	for i := range expected {
		expected[i].UpdatePrecedence() // to match what is returned...
	}
	idx, actual, err := s.Intentions(nil)
	assert.NoError(err)
	assert.Equal(idx, uint64(2))
	assert.Equal(expected, actual)
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
			"single exact namespace/name",
			[][]string{
				{"foo", "*"},
				{"foo", "bar"},
				{"foo", "baz"}, // shouldn't match
				{"bar", "bar"}, // shouldn't match
				{"bar", "*"},   // shouldn't match
				{"*", "*"},
			},
			[][]string{
				{"foo", "bar"},
			},
			[][][]string{
				{
					{"foo", "bar"},
					{"foo", "*"},
					{"*", "*"},
				},
			},
		},

		{
			"multiple exact namespace/name",
			[][]string{
				{"foo", "*"},
				{"foo", "bar"},
				{"foo", "baz"}, // shouldn't match
				{"bar", "bar"},
				{"bar", "*"},
			},
			[][]string{
				{"foo", "bar"},
				{"bar", "bar"},
			},
			[][][]string{
				{
					{"foo", "bar"},
					{"foo", "*"},
				},
				{
					{"bar", "bar"},
					{"bar", "*"},
				},
			},
		},

		{
			"single exact namespace/name with duplicate destinations",
			[][]string{
				// 4-tuple specifies src and destination to test duplicate destinations
				// with different sources. We flip them around to test in both
				// directions. The first pair are the ones searched on in both cases so
				// the duplicates need to be there.
				{"foo", "bar", "foo", "*"},
				{"foo", "bar", "bar", "*"},
				{"*", "*", "*", "*"},
			},
			[][]string{
				{"foo", "bar"},
			},
			[][][]string{
				{
					// Note the first two have the same precedence so we rely on arbitrary
					// lexicographical tie-break behaviour.
					{"foo", "bar", "bar", "*"},
					{"foo", "bar", "foo", "*"},
					{"*", "*", "*", "*"},
				},
			},
		},
	}

	// testRunner implements the test for a single case, but can be
	// parameterized to run for both source and destination so we can
	// test both cases.
	testRunner := func(t *testing.T, tc testCase, typ structs.IntentionMatchType) {
		// Insert the set
		assert := assert.New(t)
		s := testStateStore(t)
		var idx uint64 = 1
		for _, v := range tc.Insert {
			ixn := &structs.Intention{ID: testUUID()}
			switch typ {
			case structs.IntentionMatchDestination:
				ixn.DestinationNS = v[0]
				ixn.DestinationName = v[1]
				if len(v) == 4 {
					ixn.SourceNS = v[2]
					ixn.SourceName = v[3]
				}
			case structs.IntentionMatchSource:
				ixn.SourceNS = v[0]
				ixn.SourceName = v[1]
				if len(v) == 4 {
					ixn.DestinationNS = v[2]
					ixn.DestinationName = v[3]
				}
			}

			assert.NoError(s.IntentionSet(idx, ixn))

			idx++
		}

		// Build the arguments
		args := &structs.IntentionQueryMatch{Type: typ}
		for _, q := range tc.Query {
			args.Entries = append(args.Entries, structs.IntentionMatchEntry{
				Namespace: q[0],
				Name:      q[1],
			})
		}

		// Match
		_, matches, err := s.IntentionMatch(nil, args)
		assert.NoError(err)

		// Should have equal lengths
		require.Len(t, matches, len(tc.Expected))

		// Verify matches
		for i, expected := range tc.Expected {
			var actual [][]string
			for _, ixn := range matches[i] {
				switch typ {
				case structs.IntentionMatchDestination:
					if len(expected) > 1 && len(expected[0]) == 4 {
						actual = append(actual, []string{
							ixn.DestinationNS,
							ixn.DestinationName,
							ixn.SourceNS,
							ixn.SourceName,
						})
					} else {
						actual = append(actual, []string{ixn.DestinationNS, ixn.DestinationName})
					}
				case structs.IntentionMatchSource:
					if len(expected) > 1 && len(expected[0]) == 4 {
						actual = append(actual, []string{
							ixn.SourceNS,
							ixn.SourceName,
							ixn.DestinationNS,
							ixn.DestinationName,
						})
					} else {
						actual = append(actual, []string{ixn.SourceNS, ixn.SourceName})
					}
				}
			}

			assert.Equal(expected, actual)
		}
	}

	for _, tc := range cases {
		t.Run(tc.Name+" (destination)", func(t *testing.T) {
			testRunner(t, tc, structs.IntentionMatchDestination)
		})

		t.Run(tc.Name+" (source)", func(t *testing.T) {
			testRunner(t, tc, structs.IntentionMatchSource)
		})
	}
}

func TestStore_Intention_Snapshot_Restore(t *testing.T) {
	assert := assert.New(t)
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
		assert.NoError(s.IntentionSet(uint64(4+i), ixn))
	}

	// Snapshot the queries.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	assert.NoError(s.IntentionDelete(7, ixns[0].ID))

	// Verify the snapshot.
	assert.Equal(snap.LastIndex(), uint64(6))

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
		expected[i].UpdatePrecedence() // to match what is returned...
	}
	dump, err := snap.Intentions()
	assert.NoError(err)
	assert.Equal(expected, dump)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, ixn := range dump {
			assert.NoError(restore.Intention(ixn))
		}
		restore.Commit()

		// Read the restored values back out and verify that they match. Note that
		// Intentions are returned precedence sorted unlike the snapshot so we need
		// to rearrange the expected slice some.
		expected[0], expected[1], expected[2] = expected[1], expected[2], expected[0]
		idx, actual, err := s.Intentions(nil)
		assert.NoError(err)
		assert.Equal(idx, uint64(6))
		assert.Equal(expected, actual)
	}()
}
