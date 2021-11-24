//go:build !consulent
// +build !consulent

package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

func (g *Graveyard) insertTombstoneWithTxn(tx WriteTxn, _ string, stone *Tombstone, updateMax bool) error {
	if err := tx.Insert("tombstones", stone); err != nil {
		return err
	}

	if updateMax {
		if err := indexUpdateMaxTxn(tx, stone.Index, "tombstones"); err != nil {
			return fmt.Errorf("failed updating tombstone index: %v", err)
		}
	} else {
		if err := tx.Insert(tableIndex, &IndexEntry{"tombstones", stone.Index}); err != nil {
			return fmt.Errorf("failed updating tombstone index: %s", err)
		}
	}
	return nil
}

// GetMaxIndexTxn returns the highest index tombstone whose key matches the
// given context, using a prefix match.
func (g *Graveyard) GetMaxIndexTxn(tx ReadTxn, prefix string, _ *structs.EnterpriseMeta) (uint64, error) {
	var lindex uint64
	q := Query{Value: prefix, EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition()}
	stones, err := tx.Get(tableTombstones, indexID+"_prefix", q)
	if err != nil {
		return 0, fmt.Errorf("failed querying tombstones: %s", err)
	}
	for stone := stones.Next(); stone != nil; stone = stones.Next() {
		s := stone.(*Tombstone)
		if s.Index > lindex {
			lindex = s.Index
		}
	}
	return lindex, nil
}
