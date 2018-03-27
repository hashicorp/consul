package state

import (
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/assert"
)

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
