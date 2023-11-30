// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

const tableFederationStates = "federation-states"

func federationStateTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableFederationStates,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Datacenter",
					Lowercase: true,
				},
			},
		},
	}
}

// FederationStates is used to pull all the federation states for the snapshot.
func (s *Snapshot) FederationStates() ([]*structs.FederationState, error) {
	configs, err := s.tx.Get(tableFederationStates, "id")
	if err != nil {
		return nil, err
	}

	var ret []*structs.FederationState
	for wrapped := configs.Next(); wrapped != nil; wrapped = configs.Next() {
		ret = append(ret, wrapped.(*structs.FederationState))
	}

	return ret, nil
}

// FederationState is used when restoring from a snapshot.
func (s *Restore) FederationState(g *structs.FederationState) error {
	// Insert
	if err := s.tx.Insert(tableFederationStates, g); err != nil {
		return fmt.Errorf("failed restoring federation state object: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, g.ModifyIndex, tableFederationStates); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func (s *Store) FederationStateBatchSet(idx uint64, configs structs.FederationStates) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, config := range configs {
		if err := federationStateSetTxn(tx, idx, config); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// FederationStateSet is called to do an upsert of a given federation state.
func (s *Store) FederationStateSet(idx uint64, config *structs.FederationState) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := federationStateSetTxn(tx, idx, config); err != nil {
		return err
	}

	return tx.Commit()
}

// federationStateSetTxn upserts a federation state inside of a transaction.
func federationStateSetTxn(tx WriteTxn, idx uint64, config *structs.FederationState) error {
	if config.Datacenter == "" {
		return fmt.Errorf("missing datacenter on federation state")
	}

	// Check for existing.
	var existing *structs.FederationState
	existingRaw, err := tx.First(tableFederationStates, "id", config.Datacenter)
	if err != nil {
		return fmt.Errorf("failed federation state lookup: %s", err)
	}

	if existingRaw != nil {
		existing = existingRaw.(*structs.FederationState)
	}

	// Set the indexes
	if existing != nil {
		config.CreateIndex = existing.CreateIndex
		config.ModifyIndex = idx
	} else {
		config.CreateIndex = idx
		config.ModifyIndex = idx
	}

	if config.PrimaryModifyIndex == 0 {
		// Since replication ordinarily would set this value for us, we can
		// assume this is a write to the primary datacenter's federation state
		// so we can just duplicate the new modify index.
		config.PrimaryModifyIndex = idx
	}

	// Insert the federation state and update the index
	if err := tx.Insert(tableFederationStates, config); err != nil {
		return fmt.Errorf("failed inserting federation state: %s", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableFederationStates, idx}); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	return nil
}

// FederationStateGet is called to get a federation state.
func (s *Store) FederationStateGet(ws memdb.WatchSet, datacenter string) (uint64, *structs.FederationState, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return federationStateGetTxn(tx, ws, datacenter)
}

func federationStateGetTxn(tx ReadTxn, ws memdb.WatchSet, datacenter string) (uint64, *structs.FederationState, error) {
	// Get the index
	idx := maxIndexTxn(tx, tableFederationStates)

	// Get the existing contents.
	watchCh, existing, err := tx.FirstWatch(tableFederationStates, "id", datacenter)
	if err != nil {
		return 0, nil, fmt.Errorf("failed federation state lookup: %s", err)
	}
	ws.Add(watchCh)

	if existing == nil {
		return idx, nil, nil
	}

	config, ok := existing.(*structs.FederationState)
	if !ok {
		return 0, nil, fmt.Errorf("federation state %q is an invalid type: %T", datacenter, config)
	}

	return idx, config, nil
}

// FederationStateList is called to get all federation state objects.
func (s *Store) FederationStateList(ws memdb.WatchSet) (uint64, []*structs.FederationState, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return federationStateListTxn(tx, ws)
}

func federationStateListTxn(tx ReadTxn, ws memdb.WatchSet) (uint64, []*structs.FederationState, error) {
	// Get the index
	idx := maxIndexTxn(tx, tableFederationStates)

	iter, err := tx.Get(tableFederationStates, "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed federation state lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results []*structs.FederationState
	for v := iter.Next(); v != nil; v = iter.Next() {
		results = append(results, v.(*structs.FederationState))
	}
	return idx, results, nil
}

func (s *Store) FederationStateDelete(idx uint64, datacenter string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := federationStateDeleteTxn(tx, idx, datacenter); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) FederationStateBatchDelete(idx uint64, datacenters []string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	for _, datacenter := range datacenters {
		if err := federationStateDeleteTxn(tx, idx, datacenter); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func federationStateDeleteTxn(tx WriteTxn, idx uint64, datacenter string) error {
	// Try to retrieve the existing federation state.
	existing, err := tx.First(tableFederationStates, "id", datacenter)
	if err != nil {
		return fmt.Errorf("failed federation state lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	// Delete the federation state from the DB and update the index.
	if err := tx.Delete(tableFederationStates, existing); err != nil {
		return fmt.Errorf("failed removing federation state: %s", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableFederationStates, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}
