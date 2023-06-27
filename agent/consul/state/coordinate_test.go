// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/serf/coordinate"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
)

// TODO(partitions): test partitioned nodes here

// generateRandomCoordinate creates a random coordinate. This mucks with the
// underlying structure directly, so it's not really useful for any particular
// position in the network, but it's a good payload to send through to make
// sure things come out the other side or get stored correctly.
func generateRandomCoordinate() *coordinate.Coordinate {
	config := coordinate.DefaultConfig()
	coord := coordinate.NewCoordinate(config)
	for i := range coord.Vec {
		coord.Vec[i] = rand.NormFloat64()
	}
	coord.Error = rand.NormFloat64()
	coord.Adjustment = rand.NormFloat64()
	return coord
}

func TestStateStore_Coordinate_Updates(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s := testStateStore(t)

	// Make sure the coordinates list starts out empty, and that a query for
	// a per-node coordinate for a nonexistent node doesn't do anything bad.
	ws := memdb.NewWatchSet()
	idx, all, err := s.Coordinates(ws, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), idx, "bad index")
	require.Nil(t, all)

	coordinateWs := memdb.NewWatchSet()
	_, coords, err := s.Coordinate(coordinateWs, "nope", nil)
	require.NoError(t, err)
	require.Equal(t, lib.CoordinateSet{}, coords)

	// Make an update for nodes that don't exist and make sure they get
	// ignored.
	updates := structs.Coordinates{
		&structs.Coordinate{
			Node:  "node1",
			Coord: generateRandomCoordinate(),
		},
		&structs.Coordinate{
			Node:  "node2",
			Coord: generateRandomCoordinate(),
		},
	}
	require.NoError(t, s.CoordinateBatchUpdate(1, updates))
	require.False(t, watchFired(ws) || watchFired(coordinateWs))

	// Should still be empty, though applying an empty batch does NOT bump
	// the table index.
	ws = memdb.NewWatchSet()
	idx, all, err = s.Coordinates(ws, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), idx, "bad index")
	require.Nil(t, all)

	coordinateWs = memdb.NewWatchSet()
	idx, _, err = s.Coordinate(coordinateWs, "node1", nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), idx, "bad index")

	// Register the nodes then do the update again.
	testRegisterNode(t, s, 1, "node1")
	testRegisterNode(t, s, 2, "node2")
	require.NoError(t, s.CoordinateBatchUpdate(3, updates))
	if !watchFired(ws) || !watchFired(coordinateWs) {
		t.Fatalf("bad")
	}

	// Should go through now.
	ws = memdb.NewWatchSet()
	idx, all, err = s.Coordinates(ws, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), idx, "bad index")
	require.Equal(t, updates, all)

	// Also verify the per-node coordinate interface.
	nodeWs := make([]memdb.WatchSet, len(updates))
	for i, update := range updates {
		nodeWs[i] = memdb.NewWatchSet()
		idx, coords, err := s.Coordinate(nodeWs[i], update.Node, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx, "bad index")
		expected := lib.CoordinateSet{
			"": update.Coord,
		}
		require.Equal(t, expected, coords)
	}

	// Update the coordinate for one of the nodes.
	updates[1].Coord = generateRandomCoordinate()
	require.NoError(t, s.CoordinateBatchUpdate(4, updates))
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
	for _, ws := range nodeWs {
		if !watchFired(ws) {
			t.Fatalf("bad")
		}
	}

	// Verify it got applied.
	idx, all, err = s.Coordinates(nil, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(4), idx, "bad index")
	require.Equal(t, updates, all)

	// And check the per-node coordinate version of the same thing.
	for _, update := range updates {
		idx, coords, err := s.Coordinate(nil, update.Node, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(4), idx, "bad index")
		expected := lib.CoordinateSet{
			"": update.Coord,
		}
		require.Equal(t, expected, coords)
	}

	// Apply an invalid update and make sure it gets ignored.
	badUpdates := structs.Coordinates{
		&structs.Coordinate{
			Node:  "node1",
			Coord: &coordinate.Coordinate{Height: math.NaN()},
		},
	}
	require.NoError(t, s.CoordinateBatchUpdate(5, badUpdates))

	// Verify we are at the previous state, and verify that the empty batch
	// does NOT bump the table index.
	idx, all, err = s.Coordinates(nil, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(4), idx, "bad index")
	require.Equal(t, updates, all)
}

func TestStateStore_Coordinate_Cleanup(t *testing.T) {
	s := testStateStore(t)

	// Register a node and update its coordinate.
	testRegisterNode(t, s, 1, "node1")
	updates := structs.Coordinates{
		&structs.Coordinate{
			Node:    "node1",
			Segment: "alpha",
			Coord:   generateRandomCoordinate(),
		},
		&structs.Coordinate{
			Node:    "node1",
			Segment: "beta",
			Coord:   generateRandomCoordinate(),
		},
	}
	require.NoError(t, s.CoordinateBatchUpdate(2, updates))

	// Make sure it's in there.
	_, coords, err := s.Coordinate(nil, "node1", nil)
	require.NoError(t, err)
	expected := lib.CoordinateSet{
		"alpha": updates[0].Coord,
		"beta":  updates[1].Coord,
	}
	require.Equal(t, expected, coords)

	// Now delete the node.
	require.NoError(t, s.DeleteNode(3, "node1", nil, ""))

	// Make sure the coordinate is gone.
	_, coords, err = s.Coordinate(nil, "node1", nil)
	require.NoError(t, err)
	require.Equal(t, lib.CoordinateSet{}, coords)

	// Make sure the index got updated.
	idx, all, err := s.Coordinates(nil, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), idx, "bad index")
	require.Nil(t, all)
}

func TestStateStore_Coordinate_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)

	// Register two nodes and update their coordinates.
	testRegisterNode(t, s, 1, "node1")
	testRegisterNode(t, s, 2, "node2")
	updates := structs.Coordinates{
		&structs.Coordinate{
			Node:  "node1",
			Coord: generateRandomCoordinate(),
		},
		&structs.Coordinate{
			Node:  "node2",
			Coord: generateRandomCoordinate(),
		},
	}
	require.NoError(t, s.CoordinateBatchUpdate(3, updates))

	// Manually put a bad coordinate in for node3.
	testRegisterNode(t, s, 4, "node3")
	badUpdate := &structs.Coordinate{
		Node:  "node3",
		Coord: &coordinate.Coordinate{Height: math.NaN()},
	}
	tx := s.db.WriteTxn(5)
	require.NoError(t, tx.Insert("coordinates", badUpdate))
	require.NoError(t, tx.Commit())

	// Snapshot the coordinates.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	trash := structs.Coordinates{
		&structs.Coordinate{
			Node:  "node1",
			Coord: generateRandomCoordinate(),
		},
		&structs.Coordinate{
			Node:  "node2",
			Coord: generateRandomCoordinate(),
		},
	}
	require.NoError(t, s.CoordinateBatchUpdate(5, trash))

	// Verify the snapshot.
	require.Equal(t, uint64(4), snap.LastIndex(), "bad index")
	iter, err := snap.Coordinates()
	require.NoError(t, err)

	var dump structs.Coordinates
	for coord := iter.Next(); coord != nil; coord = iter.Next() {
		dump = append(dump, coord.(*structs.Coordinate))
	}

	// The snapshot will have the bad update in it, since we don't filter on
	// the read side.
	require.Equal(t, append(updates, badUpdate), dump)

	testutil.RunStep(t, "restore the values into a new state store", func(t *testing.T) {
		s := testStateStore(t)
		restore := s.Restore()
		require.NoError(t, restore.Coordinates(6, dump))
		restore.Commit()

		// Read the restored coordinates back out and verify that they match.
		idx, res, err := s.Coordinates(nil, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(6), idx, "bad index")
		require.Equal(t, updates, res)

		// Check that the index was updated (note that it got passed
		// in during the restore).
		require.Equal(t, uint64(6), s.maxIndex("coordinates"), "bad index")
	})
}
