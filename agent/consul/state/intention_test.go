package state

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

func TestStore_IntentionGet_none(t *testing.T) {
	s := testStateStore(t)

	// Querying with no results returns nil.
	ws := memdb.NewWatchSet()
	idx, res, err := s.IntentionGet(ws, testUUID())
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}
}

func TestStore_IntentionSetGet_basic(t *testing.T) {
	s := testStateStore(t)

	// Call Get to populate the watch set
	ws := memdb.NewWatchSet()
	_, _, err := s.IntentionGet(ws, testUUID())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Build a valid intention
	ixn := &structs.Intention{
		ID: testUUID(),
	}

	// Inserting a with empty ID is disallowed.
	if err := s.IntentionSet(1, ixn); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex(intentionsTableName); idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Read it back out and verify it.
	expected := &structs.Intention{
		ID: ixn.ID,
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}

	ws = memdb.NewWatchSet()
	idx, actual, err := s.IntentionGet(ws, ixn.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != expected.CreateIndex {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}

	// Change a value and test updating
	ixn.SourceNS = "foo"
	if err := s.IntentionSet(2, ixn); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex(intentionsTableName); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Read it back and verify the data was updated
	expected.SourceNS = ixn.SourceNS
	expected.ModifyIndex = 2
	ws = memdb.NewWatchSet()
	idx, actual, err = s.IntentionGet(ws, ixn.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != expected.ModifyIndex {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestStore_IntentionSet_emptyId(t *testing.T) {
	s := testStateStore(t)

	ws := memdb.NewWatchSet()
	_, _, err := s.IntentionGet(ws, testUUID())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Inserting a with empty ID is disallowed.
	if err := s.IntentionSet(1, &structs.Intention{}); err == nil {
		t.Fatalf("expected %#v, got: %#v", ErrMissingIntentionID, err)
	}

	// Index is not updated if nothing is saved.
	if idx := s.maxIndex(intentionsTableName); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStore_IntentionSet_updateCreatedAt(t *testing.T) {
	s := testStateStore(t)

	// Build a valid intention
	now := time.Now()
	ixn := structs.Intention{
		ID:        testUUID(),
		CreatedAt: now,
	}

	// Insert
	if err := s.IntentionSet(1, &ixn); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Change a value and test updating
	ixnUpdate := ixn
	ixnUpdate.CreatedAt = now.Add(10 * time.Second)
	if err := s.IntentionSet(2, &ixnUpdate); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Read it back and verify
	_, actual, err := s.IntentionGet(nil, ixn.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !actual.CreatedAt.Equal(now) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestStore_IntentionDelete(t *testing.T) {
	s := testStateStore(t)

	// Call Get to populate the watch set
	ws := memdb.NewWatchSet()
	_, _, err := s.IntentionGet(ws, testUUID())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Create
	ixn := &structs.Intention{ID: testUUID()}
	if err := s.IntentionSet(1, ixn); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex(intentionsTableName); idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Delete
	if err := s.IntentionDelete(2, ixn.ID); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex(intentionsTableName); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Sanity check to make sure it's not there.
	idx, actual, err := s.IntentionGet(nil, ixn.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
	if actual != nil {
		t.Fatalf("bad: %v", actual)
	}
}

func TestStore_IntentionsList(t *testing.T) {
	s := testStateStore(t)

	// Querying with no results returns nil.
	ws := memdb.NewWatchSet()
	idx, res, err := s.Intentions(ws)
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Create some intentions
	ixns := structs.Intentions{
		&structs.Intention{
			ID: testUUID(),
		},
		&structs.Intention{
			ID: testUUID(),
		},
	}

	// Force deterministic sort order
	ixns[0].ID = "a" + ixns[0].ID[1:]
	ixns[1].ID = "b" + ixns[1].ID[1:]

	// Create
	for i, ixn := range ixns {
		if err := s.IntentionSet(uint64(1+i), ixn); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Read it back and verify.
	expected := structs.Intentions{
		&structs.Intention{
			ID: ixns[0].ID,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		&structs.Intention{
			ID: ixns[1].ID,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		},
	}
	idx, actual, err := s.Intentions(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}
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
	}

	// testRunner implements the test for a single case, but can be
	// parameterized to run for both source and destination so we can
	// test both cases.
	testRunner := func(t *testing.T, tc testCase, typ structs.IntentionMatchType) {
		// Insert the set
		s := testStateStore(t)
		var idx uint64 = 1
		for _, v := range tc.Insert {
			ixn := &structs.Intention{ID: testUUID()}
			switch typ {
			case structs.IntentionMatchDestination:
				ixn.DestinationNS = v[0]
				ixn.DestinationName = v[1]
			case structs.IntentionMatchSource:
				ixn.SourceNS = v[0]
				ixn.SourceName = v[1]
			}

			err := s.IntentionSet(idx, ixn)
			if err != nil {
				t.Fatalf("error inserting: %s", err)
			}

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
		if err != nil {
			t.Fatalf("error matching: %s", err)
		}

		// Should have equal lengths
		if len(matches) != len(tc.Expected) {
			t.Fatalf("bad (got, wanted):\n\n%#v\n\n%#v", tc.Expected, matches)
		}

		// Verify matches
		for i, expected := range tc.Expected {
			var actual [][]string
			for _, ixn := range matches[i] {
				switch typ {
				case structs.IntentionMatchDestination:
					actual = append(actual, []string{ixn.DestinationNS, ixn.DestinationName})
				case structs.IntentionMatchSource:
					actual = append(actual, []string{ixn.SourceNS, ixn.SourceName})
				}
			}

			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("bad (got, wanted):\n\n%#v\n\n%#v", actual, expected)
			}
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
