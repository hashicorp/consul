package state

import (
	"reflect"
	"testing"

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
