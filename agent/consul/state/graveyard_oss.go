// +build !consulent

package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
)

func (g *Graveyard) insertTombstoneWithTxn(tx *memdb.Txn,
	table string, stone *Tombstone, updateMax bool) error {

	if err := tx.Insert("tombstones", stone); err != nil {
		return err
	}

	if updateMax {
		if err := indexUpdateMaxTxn(tx, stone.Index, "tombstones"); err != nil {
			return fmt.Errorf("failed updating tombstone index: %v", err)
		}
	} else {
		if err := tx.Insert("index", &IndexEntry{"tombstones", stone.Index}); err != nil {
			return fmt.Errorf("failed updating tombstone index: %s", err)
		}
	}
	return nil
}
