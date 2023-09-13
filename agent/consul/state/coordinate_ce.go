//go:build !consulent
// +build !consulent

package state

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func coordinatesMaxIndex(tx ReadTxn, entMeta *acl.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableCoordinates)
}

func updateCoordinatesIndexes(tx WriteTxn, idx uint64, entMeta *acl.EnterpriseMeta) error {
	// Update the index.
	if err := indexUpdateMaxTxn(tx, idx, tableCoordinates); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

func ensureCoordinateTxn(tx WriteTxn, idx uint64, coord *structs.Coordinate) error {
	// ensure that the Partition is always empty within the state store
	coord.Partition = ""

	if err := tx.Insert(tableCoordinates, coord); err != nil {
		return fmt.Errorf("failed inserting coordinate: %s", err)
	}

	if err := updateCoordinatesIndexes(tx, idx, coord.GetEnterpriseMeta()); err != nil {
		return fmt.Errorf("failed updating coordinate index: %s", err)
	}

	return nil
}
