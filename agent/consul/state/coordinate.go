// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

const tableCoordinates = "coordinates"

func indexFromCoordinate(c *structs.Coordinate) ([]byte, error) {
	if c.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(c.Node))
	b.String(strings.ToLower(c.Segment))
	return b.Bytes(), nil
}

func indexNodeFromCoordinate(c *structs.Coordinate) ([]byte, error) {
	if c.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(c.Node))
	return b.Bytes(), nil
}

func indexFromCoordinateQuery(q CoordinateQuery) ([]byte, error) {
	if q.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(q.Node))
	b.String(strings.ToLower(q.Segment))
	return b.Bytes(), nil
}

type CoordinateQuery struct {
	Node      string
	Segment   string
	Partition string
}

func (c CoordinateQuery) PartitionOrDefault() string {
	return acl.PartitionOrDefault(c.Partition)
}

// coordinatesTableSchema returns a new table schema used for storing
// network coordinates.
func coordinatesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableCoordinates,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[CoordinateQuery, *structs.Coordinate, any]{
					readIndex:   indexFromCoordinateQuery,
					writeIndex:  indexFromCoordinate,
					prefixIndex: prefixIndexFromQueryNoNamespace,
				},
			},
			indexNode: {
				Name:         indexNode,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle[Query, *structs.Coordinate]{
					readIndex:  indexFromQuery,
					writeIndex: indexNodeFromCoordinate,
				},
			},
		},
	}
}

// Coordinates is used to pull all the coordinates from the snapshot.
func (s *Snapshot) Coordinates() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableCoordinates, indexID)
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

		if err := ensureCoordinateTxn(s.tx, idx, update); err != nil {
			return fmt.Errorf("failed restoring coordinate: %s", err)
		}
	}

	return nil
}

// Coordinate returns a map of coordinates for the given node, indexed by
// network segment.
func (s *Store) Coordinate(ws memdb.WatchSet, node string, entMeta *acl.EnterpriseMeta) (uint64, lib.CoordinateSet, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	tableIdx := coordinatesMaxIndex(tx, entMeta)

	iter, err := tx.Get(tableCoordinates, indexNode, Query{
		Value:          node,
		EnterpriseMeta: *entMeta,
	})
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
func (s *Store) Coordinates(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, structs.Coordinates, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Get the table index.
	idx := coordinatesMaxIndex(tx, entMeta)

	// Pull all the coordinates.
	iter, err := tx.Get(tableCoordinates, indexID+"_prefix", entMeta)
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
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Upsert the coordinates.
	for _, update := range updates {
		// Skip any bad data that may have gotten into the database from
		// a bad client in the past.
		if !update.Coord.IsValid() {
			continue
		}

		entMeta := update.GetEnterpriseMeta()

		// Since the cleanup of coordinates is tied to deletion of
		// nodes, we silently drop any updates for nodes that we don't
		// know about. This might be possible during normal operation
		// if we happen to get a coordinate update for a node that
		// hasn't been able to add itself to the catalog yet. Since we
		// don't carefully sequence this, and since it will fix itself
		// on the next coordinate update from that node, we don't return
		// an error or log anything.
		node, err := tx.First(tableNodes, indexID, Query{
			Value:          update.Node,
			EnterpriseMeta: *entMeta,
		})
		if err != nil {
			return fmt.Errorf("failed node lookup: %s", err)
		}
		if node == nil {
			continue
		}

		if err := ensureCoordinateTxn(tx, idx, update); err != nil {
			return fmt.Errorf("failed inserting coordinate: %s", err)
		}
	}

	return tx.Commit()
}

func deleteCoordinateTxn(tx WriteTxn, idx uint64, coord *structs.Coordinate) error {
	if err := tx.Delete(tableCoordinates, coord); err != nil {
		return fmt.Errorf("failed deleting coordinate: %s", err)
	}

	if err := updateCoordinatesIndexes(tx, idx, coord.GetEnterpriseMeta()); err != nil {
		return fmt.Errorf("failed updating coordinate index: %s", err)
	}

	return nil
}
