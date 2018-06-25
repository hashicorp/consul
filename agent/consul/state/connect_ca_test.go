package state

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
	"github.com/pascaldekloe/goe/verify"
	"github.com/stretchr/testify/assert"
)

func TestStore_CAConfig(t *testing.T) {
	s := testStateStore(t)

	expected := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey":     "asdf",
			"RootCert":       "qwer",
			"RotationPeriod": 90 * 24 * time.Hour,
		},
	}

	if err := s.CASetConfig(0, expected); err != nil {
		t.Fatal(err)
	}

	idx, config, err := s.CAConfig()
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Fatalf("bad: %d", idx)
	}
	if !reflect.DeepEqual(expected, config) {
		t.Fatalf("bad: %#v, %#v", expected, config)
	}
}

func TestStore_CAConfigCAS(t *testing.T) {
	s := testStateStore(t)

	expected := &structs.CAConfiguration{
		Provider: "consul",
	}

	if err := s.CASetConfig(0, expected); err != nil {
		t.Fatal(err)
	}
	// Do an extra operation to move the index up by 1 for the
	// check-and-set operation after this
	if err := s.CASetConfig(1, expected); err != nil {
		t.Fatal(err)
	}

	// Do a CAS with an index lower than the entry
	ok, err := s.CACheckAndSetConfig(2, 0, &structs.CAConfiguration{
		Provider: "static",
	})
	if ok || err != nil {
		t.Fatalf("expected (false, nil), got: (%v, %#v)", ok, err)
	}

	// Check that the index is untouched and the entry
	// has not been updated.
	idx, config, err := s.CAConfig()
	if err != nil {
		t.Fatal(err)
	}
	if idx != 1 {
		t.Fatalf("bad: %d", idx)
	}
	if config.Provider != "consul" {
		t.Fatalf("bad: %#v", config)
	}

	// Do another CAS, this time with the correct index
	ok, err = s.CACheckAndSetConfig(2, 1, &structs.CAConfiguration{
		Provider: "static",
	})
	if !ok || err != nil {
		t.Fatalf("expected (true, nil), got: (%v, %#v)", ok, err)
	}

	// Make sure the config was updated
	idx, config, err = s.CAConfig()
	if err != nil {
		t.Fatal(err)
	}
	if idx != 2 {
		t.Fatalf("bad: %d", idx)
	}
	if config.Provider != "static" {
		t.Fatalf("bad: %#v", config)
	}
}

func TestStore_CAConfig_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)
	before := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey":     "asdf",
			"RootCert":       "qwer",
			"RotationPeriod": 90 * 24 * time.Hour,
		},
	}
	if err := s.CASetConfig(99, before); err != nil {
		t.Fatal(err)
	}

	snap := s.Snapshot()
	defer snap.Close()

	after := &structs.CAConfiguration{
		Provider: "static",
		Config:   map[string]interface{}{},
	}
	if err := s.CASetConfig(100, after); err != nil {
		t.Fatal(err)
	}

	snapped, err := snap.CAConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	verify.Values(t, "", before, snapped)

	s2 := testStateStore(t)
	restore := s2.Restore()
	if err := restore.CAConfig(snapped); err != nil {
		t.Fatalf("err: %s", err)
	}
	restore.Commit()

	idx, res, err := s2.CAConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 99 {
		t.Fatalf("bad index: %d", idx)
	}
	verify.Values(t, "", before, res)
}

func TestStore_CARootSetList(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Call list to populate the watch set
	ws := memdb.NewWatchSet()
	_, _, err := s.CARoots(ws)
	assert.Nil(err)

	// Build a valid value
	ca1 := connect.TestCA(t, nil)

	// Set
	ok, err := s.CARootSetCAS(1, 0, []*structs.CARoot{ca1})
	assert.Nil(err)
	assert.True(ok)

	// Make sure the index got updated.
	assert.Equal(s.maxIndex(caRootTableName), uint64(1))
	assert.True(watchFired(ws), "watch fired")

	// Read it back out and verify it.
	expected := *ca1
	expected.RaftIndex = structs.RaftIndex{
		CreateIndex: 1,
		ModifyIndex: 1,
	}

	ws = memdb.NewWatchSet()
	_, roots, err := s.CARoots(ws)
	assert.Nil(err)
	assert.Len(roots, 1)
	actual := roots[0]
	assert.Equal(&expected, actual)
}

func TestStore_CARootSet_emptyID(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Call list to populate the watch set
	ws := memdb.NewWatchSet()
	_, _, err := s.CARoots(ws)
	assert.Nil(err)

	// Build a valid value
	ca1 := connect.TestCA(t, nil)
	ca1.ID = ""

	// Set
	ok, err := s.CARootSetCAS(1, 0, []*structs.CARoot{ca1})
	assert.NotNil(err)
	assert.Contains(err.Error(), ErrMissingCARootID.Error())
	assert.False(ok)

	// Make sure the index got updated.
	assert.Equal(s.maxIndex(caRootTableName), uint64(0))
	assert.False(watchFired(ws), "watch fired")

	// Read it back out and verify it.
	ws = memdb.NewWatchSet()
	_, roots, err := s.CARoots(ws)
	assert.Nil(err)
	assert.Len(roots, 0)
}

func TestStore_CARootSet_noActive(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Call list to populate the watch set
	ws := memdb.NewWatchSet()
	_, _, err := s.CARoots(ws)
	assert.Nil(err)

	// Build a valid value
	ca1 := connect.TestCA(t, nil)
	ca1.Active = false
	ca2 := connect.TestCA(t, nil)
	ca2.Active = false

	// Set
	ok, err := s.CARootSetCAS(1, 0, []*structs.CARoot{ca1, ca2})
	assert.NotNil(err)
	assert.Contains(err.Error(), "exactly one active")
	assert.False(ok)
}

func TestStore_CARootSet_multipleActive(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Call list to populate the watch set
	ws := memdb.NewWatchSet()
	_, _, err := s.CARoots(ws)
	assert.Nil(err)

	// Build a valid value
	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)

	// Set
	ok, err := s.CARootSetCAS(1, 0, []*structs.CARoot{ca1, ca2})
	assert.NotNil(err)
	assert.Contains(err.Error(), "exactly one active")
	assert.False(ok)
}

func TestStore_CARootActive_valid(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Build a valid value
	ca1 := connect.TestCA(t, nil)
	ca1.Active = false
	ca2 := connect.TestCA(t, nil)
	ca3 := connect.TestCA(t, nil)
	ca3.Active = false

	// Set
	ok, err := s.CARootSetCAS(1, 0, []*structs.CARoot{ca1, ca2, ca3})
	assert.Nil(err)
	assert.True(ok)

	// Query
	ws := memdb.NewWatchSet()
	idx, res, err := s.CARootActive(ws)
	assert.Equal(idx, uint64(1))
	assert.Nil(err)
	assert.NotNil(res)
	assert.Equal(ca2.ID, res.ID)
}

// Test that querying the active CA returns the correct value.
func TestStore_CARootActive_none(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Querying with no results returns nil.
	ws := memdb.NewWatchSet()
	idx, res, err := s.CARootActive(ws)
	assert.Equal(idx, uint64(0))
	assert.Nil(res)
	assert.Nil(err)
}

func TestStore_CARoot_Snapshot_Restore(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Create some intentions.
	roots := structs.CARoots{
		connect.TestCA(t, nil),
		connect.TestCA(t, nil),
		connect.TestCA(t, nil),
	}
	for _, r := range roots[1:] {
		r.Active = false
	}

	// Force the sort order of the UUIDs before we create them so the
	// order is deterministic.
	id := testUUID()
	roots[0].ID = "a" + id[1:]
	roots[1].ID = "b" + id[1:]
	roots[2].ID = "c" + id[1:]

	// Now create
	ok, err := s.CARootSetCAS(1, 0, roots)
	assert.Nil(err)
	assert.True(ok)

	// Snapshot the queries.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	ok, err = s.CARootSetCAS(2, 1, roots[:1])
	assert.Nil(err)
	assert.True(ok)

	// Verify the snapshot.
	assert.Equal(snap.LastIndex(), uint64(1))
	dump, err := snap.CARoots()
	assert.Nil(err)
	assert.Equal(roots, dump)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, r := range dump {
			assert.Nil(restore.CARoot(r))
		}
		restore.Commit()

		// Read the restored values back out and verify that they match.
		idx, actual, err := s.CARoots(nil)
		assert.Nil(err)
		assert.Equal(idx, uint64(2))
		assert.Equal(roots, actual)
	}()
}

func TestStore_CABuiltinProvider(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	{
		expected := &structs.CAConsulProviderState{
			ID:         "foo",
			PrivateKey: "a",
			RootCert:   "b",
		}

		ok, err := s.CASetProviderState(0, expected)
		assert.NoError(err)
		assert.True(ok)

		idx, state, err := s.CAProviderState(expected.ID)
		assert.NoError(err)
		assert.Equal(idx, uint64(0))
		assert.Equal(expected, state)
	}

	{
		expected := &structs.CAConsulProviderState{
			ID:         "bar",
			PrivateKey: "c",
			RootCert:   "d",
		}

		ok, err := s.CASetProviderState(1, expected)
		assert.NoError(err)
		assert.True(ok)

		idx, state, err := s.CAProviderState(expected.ID)
		assert.NoError(err)
		assert.Equal(idx, uint64(1))
		assert.Equal(expected, state)
	}
}

func TestStore_CABuiltinProvider_Snapshot_Restore(t *testing.T) {
	assert := assert.New(t)
	s := testStateStore(t)

	// Create multiple state entries.
	before := []*structs.CAConsulProviderState{
		{
			ID:         "bar",
			PrivateKey: "y",
			RootCert:   "z",
		},
		{
			ID:         "foo",
			PrivateKey: "a",
			RootCert:   "b",
		},
	}

	for i, state := range before {
		ok, err := s.CASetProviderState(uint64(98+i), state)
		assert.NoError(err)
		assert.True(ok)
	}

	// Take a snapshot.
	snap := s.Snapshot()
	defer snap.Close()

	// Modify the state store.
	after := &structs.CAConsulProviderState{
		ID:         "foo",
		PrivateKey: "c",
		RootCert:   "d",
	}
	ok, err := s.CASetProviderState(100, after)
	assert.NoError(err)
	assert.True(ok)

	snapped, err := snap.CAProviderState()
	assert.NoError(err)
	assert.Equal(before, snapped)

	// Restore onto a new state store.
	s2 := testStateStore(t)
	restore := s2.Restore()
	for _, entry := range snapped {
		assert.NoError(restore.CAProviderState(entry))
	}
	restore.Commit()

	// Verify the restored values match those from before the snapshot.
	for _, state := range before {
		idx, res, err := s2.CAProviderState(state.ID)
		assert.NoError(err)
		assert.Equal(idx, uint64(99))
		assert.Equal(state, res)
	}
}
