// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
)

// Tombstone is the internal type used to track tombstones.
type Tombstone struct {
	Key   string
	Index uint64

	acl.EnterpriseMeta
}

func (t Tombstone) IDValue() string {
	return t.Key
}

// Graveyard manages a set of tombstones.
type Graveyard struct {
	// GC is when we create tombstones to track their time-to-live.
	// The GC is consumed upstream to manage clearing of tombstones.
	gc *TombstoneGC
}

// NewGraveyard returns a new graveyard.
func NewGraveyard(gc *TombstoneGC) *Graveyard {
	return &Graveyard{gc: gc}
}

// InsertTxn adds a new tombstone.
func (g *Graveyard) InsertTxn(tx WriteTxn, key string, idx uint64, entMeta *acl.EnterpriseMeta) error {
	stone := &Tombstone{
		Key:   key,
		Index: idx,
	}
	if entMeta != nil {
		stone.EnterpriseMeta = *entMeta
	}

	// Insert the tombstone.
	if err := g.insertTombstoneWithTxn(tx, "tombstones", stone, false); err != nil {
		return fmt.Errorf("failed inserting tombstone: %s", err)
	}

	// If GC is configured, then we hint that this index requires reaping.
	if g.gc != nil {
		tx.Defer(func() { g.gc.Hint(idx) })
	}
	return nil
}

// DumpTxn returns all the tombstones.
func (g *Graveyard) DumpTxn(tx ReadTxn) (memdb.ResultIterator, error) {
	return tx.Get(tableTombstones, indexID)
}

// RestoreTxn is used when restoring from a snapshot. For general inserts, use
// InsertTxn.
func (g *Graveyard) RestoreTxn(tx WriteTxn, stone *Tombstone) error {
	if err := g.insertTombstoneWithTxn(tx, "tombstones", stone, true); err != nil {
		return fmt.Errorf("failed inserting tombstone: %s", err)
	}

	return nil
}

// ReapTxn cleans out all tombstones whose index values are less than or equal
// to the given idx. This prevents unbounded storage growth of the tombstones.
func (g *Graveyard) ReapTxn(tx WriteTxn, idx uint64) error {
	// This does a full table scan since we currently can't index on a
	// numeric value. Since this is all in-memory and done infrequently
	// this pretty reasonable.
	stones, err := tx.Get(tableTombstones, indexID)
	if err != nil {
		return fmt.Errorf("failed querying tombstones: %s", err)
	}

	// Find eligible tombstones.
	var objs []interface{}
	for stone := stones.Next(); stone != nil; stone = stones.Next() {
		if stone.(*Tombstone).Index <= idx {
			objs = append(objs, stone)
		}
	}

	// Delete the tombstones in a separate loop so we don't trash the
	// iterator.
	for _, obj := range objs {
		if err := tx.Delete("tombstones", obj); err != nil {
			return fmt.Errorf("failed deleting tombstone: %s", err)
		}
	}
	return nil
}
