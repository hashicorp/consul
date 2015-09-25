package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
)

// tombstone is the internal type used to track tombstones.
type Tombstone struct {
	Key   string
	Index uint64
}

// Graveyard manages a set of tombstones for a table. This is just used for
// KVS right now but we've broken it out for other table types later.
type Graveyard struct {
	Table string
}

// NewGraveyard returns a new graveyard.
func NewGraveyard(table string) *Graveyard {
	return &Graveyard{Table: "tombstones_" + table}
}

// InsertTxn adds a new tombstone.
func (g *Graveyard) InsertTxn(tx *memdb.Txn, context string, idx uint64) error {
	stone := &Tombstone{Key: context, Index: idx}
	if err := tx.Insert(g.Table, stone); err != nil {
		return fmt.Errorf("failed inserting tombstone: %s", err)
	}

	if err := tx.Insert("index", &IndexEntry{g.Table, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// GetMaxIndexTxn returns the highest index tombstone whose key matches the
// given context, using a prefix match.
func (g *Graveyard) GetMaxIndexTxn(tx *memdb.Txn, context string) (uint64, error) {
	stones, err := tx.Get(g.Table, "id", context)
	if err != nil {
		return 0, fmt.Errorf("failed querying tombstones: %s", err)
	}

	var lindex uint64
	for stone := stones.Next(); stone != nil; stone = stones.Next() {
		r := stone.(*Tombstone)
		if r.Index > lindex {
			lindex = r.Index
		}
	}
	return lindex, nil
}

// DumpTxn returns all the tombstones.
func (g *Graveyard) DumpTxn(tx *memdb.Txn) ([]*Tombstone, error) {
	stones, err := tx.Get(g.Table, "id", "")
	if err != nil {
		return nil, fmt.Errorf("failed querying tombstones: %s", err)
	}

	var dump []*Tombstone
	for stone := stones.Next(); stone != nil; stone = stones.Next() {
		dump = append(dump, stone.(*Tombstone))
	}
	return dump, nil
}

// RestoreTxn is used when restoring from a snapshot. For general inserts, use
// InsertTxn.
func (g *Graveyard) RestoreTxn(tx *memdb.Txn, stone *Tombstone) error {
	if err := tx.Insert(g.Table, stone); err != nil {
		return fmt.Errorf("failed inserting tombstone: %s", err)
	}

	if err := indexUpdateMaxTxn(tx, stone.Index, g.Table); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// ReapTxn cleans out all tombstones whose index values are less than or equal
// to the given idx. This prevents unbounded storage growth of the tombstones.
func (g *Graveyard) ReapTxn(tx *memdb.Txn, idx uint64) error {
	// This does a full table scan since we currently can't index on a
	// numeric value. Since this is all in-memory and done infrequently
	// this pretty reasonable.
	stones, err := tx.Get(g.Table, "id", "")
	if err != nil {
		return fmt.Errorf("failed querying tombstones: %s", err)
	}

	for stone := stones.Next(); stone != nil; stone = stones.Next() {
		if stone.(*Tombstone).Index <= idx {
			if err := tx.Delete(g.Table, stone); err != nil {
				return fmt.Errorf("failed deleting tombstone: %s", err)
			}
		}
	}
	return nil
}
