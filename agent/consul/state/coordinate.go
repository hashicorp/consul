package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-memdb"
)

// Coordinates is used to pull all the coordinates from the snapshot.
func (s *Snapshot) Coordinates() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("coordinates", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Coordinates is used when restoring from a snapshot. For general inserts, use
// CoordinateBatchUpdate. We do less vetting of the updates here because they
// already got checked on the way in during a batch update.
func (s *Restore) Coordinates(idx uint64, updates structs.Coordinates) error {
	for _, update := range updates {
		// Skip any bad data that may have gotten into the database from
		// a bad client in the past.
		if !update.Coord.IsValid() {
			continue
		}

		if err := s.tx.Insert("coordinates", update); err != nil {
			return fmt.Errorf("failed restoring coordinate: %s", err)
		}
	}

	if err := indexUpdateMaxTxn(s.tx, idx, "coordinates"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// Coordinate returns a map of coordinates for the given node, indexed by
// network segment.
func (s *Store) Coordinate(node string, ws memdb.WatchSet) (uint64, lib.CoordinateSet, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	tableIdx := maxIndexTxn(tx, "coordinates")

	iter, err := tx.Get("coordinates", "node", node)
	if err != nil {
		return 0, nil, fmt.Errorf("failed coordinate lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	results := make(lib.CoordinateSet)
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		coord := raw.(*structs.Coordinate)
		results[coord.Segment] = coord.Coord
	}
	return tableIdx, results, nil
}

// Coordinates queries for all nodes with coordinates.
func (s *Store) Coordinates(ws memdb.WatchSet) (uint64, structs.Coordinates, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "coordinates")

	// Pull all the coordinates.
	iter, err := tx.Get("coordinates", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed coordinate lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results structs.Coordinates
	for coord := iter.Next(); coord != nil; coord = iter.Next() {
		results = append(results, coord.(*structs.Coordinate))
	}
	return idx, results, nil
}

// CoordinateBatchUpdate processes a batch of coordinate updates and applies
// them in a single transaction.
func (s *Store) CoordinateBatchUpdate(idx uint64, updates structs.Coordinates) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Upsert the coordinates.
	for _, update := range updates {
		// Skip any bad data that may have gotten into the database from
		// a bad client in the past.
		if !update.Coord.IsValid() {
			continue
		}

		// Since the cleanup of coordinates is tied to deletion of
		// nodes, we silently drop any updates for nodes that we don't
		// know about. This might be possible during normal operation
		// if we happen to get a coordinate update for a node that
		// hasn't been able to add itself to the catalog yet. Since we
		// don't carefully sequence this, and since it will fix itself
		// on the next coordinate update from that node, we don't return
		// an error or log anything.
		node, err := tx.First("nodes", "id", update.Node)
		if err != nil {
			return fmt.Errorf("failed node lookup: %s", err)
		}
		if node == nil {
			continue
		}

		if err := tx.Insert("coordinates", update); err != nil {
			return fmt.Errorf("failed inserting coordinate: %s", err)
		}
	}

	// Update the index.
	if err := tx.Insert("index", &IndexEntry{"coordinates", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}
