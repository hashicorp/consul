// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	tableConnectCABuiltin       = "connect-ca-builtin"
	tableConnectCABuiltinSerial = "connect-ca-builtin-serial"
	tableConnectCAConfig        = "connect-ca-config"
	tableConnectCARoots         = "connect-ca-roots"
	tableConnectCALeafCerts     = "connect-ca-leaf-certs"
)

// caBuiltinProviderTableSchema returns a new table schema used for storing
// the built-in CA provider's state for connect. This is only used by
// the internal Consul CA provider.
func caBuiltinProviderTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableConnectCABuiltin,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
			},
		},
	}
}

// caConfigTableSchema returns a new table schema used for storing
// the CA config for Connect.
func caConfigTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableConnectCAConfig,
		Indexes: map[string]*memdb.IndexSchema{
			// This table only stores one row, so this just ignores the ID field
			// and always overwrites the same config object.
			"id": {
				Name:         "id",
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.ConditionalIndex{
					Conditional: func(obj interface{}) (bool, error) { return true, nil },
				},
			},
		},
	}
}

// caRootTableSchema returns a new table schema used for storing
// CA roots for Connect.
func caRootTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableConnectCARoots,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
			},
		},
	}
}

// CAConfig is used to pull the CA config from the snapshot.
func (s *Snapshot) CAConfig() (*structs.CAConfiguration, error) {
	c, err := s.tx.First(tableConnectCAConfig, "id")
	if err != nil {
		return nil, err
	}

	config, ok := c.(*structs.CAConfiguration)
	if !ok {
		return nil, nil
	}

	return config, nil
}

// CAConfig is used when restoring from a snapshot.
func (s *Restore) CAConfig(config *structs.CAConfiguration) error {
	// Don't restore a blank CA config
	// https://github.com/hashicorp/consul/issues/4954
	if config.Provider == "" {
		return nil
	}

	if err := s.tx.Insert(tableConnectCAConfig, config); err != nil {
		return fmt.Errorf("failed restoring CA config: %s", err)
	}

	return nil
}

// CAConfig is used to get the current CA configuration.
func (s *Store) CAConfig(ws memdb.WatchSet) (uint64, *structs.CAConfiguration, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return caConfigTxn(tx, ws)
}

func caConfigTxn(tx ReadTxn, ws memdb.WatchSet) (uint64, *structs.CAConfiguration, error) {
	// Get the CA config
	ch, c, err := tx.FirstWatch(tableConnectCAConfig, "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed CA config lookup: %s", err)
	}

	ws.Add(ch)

	config, ok := c.(*structs.CAConfiguration)
	if !ok {
		return 0, nil, nil
	}

	return config.ModifyIndex, config, nil
}

// CASetConfig is used to set the current CA configuration.
func (s *Store) CASetConfig(idx uint64, config *structs.CAConfiguration) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := s.caSetConfigTxn(idx, tx, config); err != nil {
		return err
	}

	return tx.Commit()
}

// CACheckAndSetConfig is used to try updating the CA configuration with a
// given Raft index. If the CAS index specified is not equal to the last observed index
// for the config, then the call will return an error,
func (s *Store) CACheckAndSetConfig(idx, cidx uint64, config *structs.CAConfiguration) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Check for an existing config
	existing, err := tx.First(tableConnectCAConfig, "id")
	if err != nil {
		return false, fmt.Errorf("failed CA config lookup: %s", err)
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	e, ok := existing.(*structs.CAConfiguration)
	if (ok && e.ModifyIndex != cidx) || (!ok && cidx != 0) {
		return false, errors.Errorf("ModifyIndex did not match existing")
	}

	if err := s.caSetConfigTxn(idx, tx, config); err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

func (s *Store) caSetConfigTxn(idx uint64, tx WriteTxn, config *structs.CAConfiguration) error {
	// Check for an existing config
	prev, err := tx.First(tableConnectCAConfig, "id")
	if err != nil {
		return fmt.Errorf("failed CA config lookup: %s", err)
	}
	// Set the indexes, prevent the cluster ID from changing.
	if prev != nil {
		existing := prev.(*structs.CAConfiguration)
		config.CreateIndex = existing.CreateIndex
		if config.ClusterID == "" {
			config.ClusterID = existing.ClusterID
		}
	} else {
		config.CreateIndex = idx
	}
	config.ModifyIndex = idx

	if err := tx.Insert(tableConnectCAConfig, config); err != nil {
		return fmt.Errorf("failed updating CA config: %s", err)
	}
	return nil
}

// CARoots is used to pull all the CA roots for the snapshot.
func (s *Snapshot) CARoots() (structs.CARoots, error) {
	ixns, err := s.tx.Get(tableConnectCARoots, "id")
	if err != nil {
		return nil, err
	}

	var ret structs.CARoots
	for wrapped := ixns.Next(); wrapped != nil; wrapped = ixns.Next() {
		ret = append(ret, wrapped.(*structs.CARoot))
	}

	return ret, nil
}

// CARoots is used when restoring from a snapshot.
func (s *Restore) CARoot(r *structs.CARoot) error {
	// Insert
	if err := s.tx.Insert(tableConnectCARoots, r); err != nil {
		return fmt.Errorf("failed restoring CA root: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, r.ModifyIndex, tableConnectCARoots); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// CARoots returns the list of all CA roots.
func (s *Store) CARoots(ws memdb.WatchSet) (uint64, structs.CARoots, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return caRootsTxn(tx, ws)
}

func caRootsTxn(tx ReadTxn, ws memdb.WatchSet) (uint64, structs.CARoots, error) {
	// Get the index
	idx := maxIndexTxn(tx, tableConnectCARoots)

	// Get all
	iter, err := tx.Get(tableConnectCARoots, "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed CA root lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results structs.CARoots
	for v := iter.Next(); v != nil; v = iter.Next() {
		results = append(results, v.(*structs.CARoot))
	}
	return idx, results, nil
}

// CARootActive returns the currently active CARoot.
func (s *Store) CARootActive(ws memdb.WatchSet) (uint64, *structs.CARoot, error) {
	// Get all the roots since there should never be that many and just
	// do the filtering in this method.
	idx, roots, err := s.CARoots(ws)
	return idx, roots.Active(), err
}

// CARootSetCAS sets the current CA root state using a check-and-set operation.
// On success, this will replace the previous set of CARoots completely with
// the given set of roots.
//
// The first boolean result returns whether the transaction succeeded or not.
func (s *Store) CARootSetCAS(idx, cidx uint64, rs []*structs.CARoot) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := caRootSetCASTxn(tx, idx, cidx, rs); err != nil {
		return false, err
	}

	err := tx.Commit()
	return err == nil, err
}

func caRootSetCASTxn(tx WriteTxn, idx, cidx uint64, rs []*structs.CARoot) error {
	// There must be exactly one active CA root.
	activeCount := 0
	for _, r := range rs {
		if r.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		return fmt.Errorf("there must be exactly one active CA")
	}

	// Get the current max index
	if midx := maxIndexTxn(tx, tableConnectCARoots); midx != cidx {
		return nil
	}

	// Go through and find any existing matching CAs so we can preserve and
	// update their Create/ModifyIndex values.
	for _, r := range rs {
		if r.ID == "" {
			return ErrMissingCARootID
		}

		existing, err := tx.First(tableConnectCARoots, "id", r.ID)
		if err != nil {
			return fmt.Errorf("failed CA root lookup: %s", err)
		}

		if existing != nil {
			r.CreateIndex = existing.(*structs.CARoot).CreateIndex
		} else {
			r.CreateIndex = idx
		}
		r.ModifyIndex = idx
	}

	// Delete all
	_, err := tx.DeleteAll(tableConnectCARoots, "id")
	if err != nil {
		return err
	}

	// Insert all
	for _, r := range rs {
		if err := tx.Insert(tableConnectCARoots, r); err != nil {
			return err
		}
	}

	// Update the index
	if err := tx.Insert(tableIndex, &IndexEntry{tableConnectCARoots, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// CAProviderState is used to pull the built-in provider states from the snapshot.
func (s *Snapshot) CAProviderState() ([]*structs.CAConsulProviderState, error) {
	ixns, err := s.tx.Get(tableConnectCABuiltin, "id")
	if err != nil {
		return nil, err
	}

	var ret []*structs.CAConsulProviderState
	for wrapped := ixns.Next(); wrapped != nil; wrapped = ixns.Next() {
		ret = append(ret, wrapped.(*structs.CAConsulProviderState))
	}

	return ret, nil
}

// CAProviderState is used when restoring from a snapshot.
func (s *Restore) CAProviderState(state *structs.CAConsulProviderState) error {
	if err := s.tx.Insert(tableConnectCABuiltin, state); err != nil {
		return fmt.Errorf("failed restoring built-in CA state: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, state.ModifyIndex, tableConnectCABuiltin); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// CAProviderState is used to get the Consul CA provider state for the given ID.
func (s *Store) CAProviderState(id string) (uint64, *structs.CAConsulProviderState, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the index
	idx := maxIndexTxn(tx, tableConnectCABuiltin)

	// Get the provider config
	c, err := tx.First(tableConnectCABuiltin, "id", id)
	if err != nil {
		return 0, nil, fmt.Errorf("failed built-in CA state lookup: %s", err)
	}

	state, ok := c.(*structs.CAConsulProviderState)
	if !ok {
		return 0, nil, nil
	}

	return idx, state, nil
}

// CASetProviderState is used to set the current built-in CA provider state.
func (s *Store) CASetProviderState(idx uint64, state *structs.CAConsulProviderState) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Check for an existing config
	existing, err := tx.First(tableConnectCABuiltin, "id", state.ID)
	if err != nil {
		return false, fmt.Errorf("failed built-in CA state lookup: %s", err)
	}

	// Set the indexes.
	if existing != nil {
		state.CreateIndex = existing.(*structs.CAConsulProviderState).CreateIndex
	} else {
		state.CreateIndex = idx
	}
	state.ModifyIndex = idx

	if err := tx.Insert(tableConnectCABuiltin, state); err != nil {
		return false, fmt.Errorf("failed updating built-in CA state: %s", err)
	}

	// Update the index
	if err := tx.Insert(tableIndex, &IndexEntry{tableConnectCABuiltin, idx}); err != nil {
		return false, fmt.Errorf("failed updating index: %s", err)
	}

	err = tx.Commit()
	return err == nil, err
}

// CADeleteProviderState is used to remove the built-in Consul CA provider
// state for the given ID.
func (s *Store) CADeleteProviderState(idx uint64, id string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Check for an existing config
	existing, err := tx.First(tableConnectCABuiltin, "id", id)
	if err != nil {
		return fmt.Errorf("failed built-in CA state lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	providerState := existing.(*structs.CAConsulProviderState)

	// Do the delete and update the index
	if err := tx.Delete(tableConnectCABuiltin, providerState); err != nil {
		return err
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableConnectCABuiltin, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return tx.Commit()
}

func (s *Store) CALeafSetIndex(idx uint64, index uint64) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	return indexUpdateMaxTxn(tx, index, tableConnectCALeafCerts)
}

func (s *Store) CARootsAndConfig(ws memdb.WatchSet) (uint64, structs.CARoots, *structs.CAConfiguration, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	confIdx, config, err := caConfigTxn(tx, ws)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed CA config lookup: %v", err)
	}

	rootsIdx, roots, err := caRootsTxn(tx, ws)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed CA roots lookup: %v", err)
	}

	idx := rootsIdx
	if confIdx > idx {
		idx = confIdx
	}

	return idx, roots, config, nil
}

func (s *Store) CAIncrementProviderSerialNumber(idx uint64) (uint64, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	existing, err := tx.First(tableIndex, "id", tableConnectCABuiltinSerial)
	if err != nil {
		return 0, fmt.Errorf("failed built-in CA serial number lookup: %s", err)
	}

	var last uint64
	if existing != nil {
		last = existing.(*IndexEntry).Value
	} else {
		// Serials used to be based on the raft indexes in the provider table,
		// so bootstrap off of that.
		last = maxIndexTxn(tx, tableConnectCABuiltin)
	}
	next := last + 1

	if err := tx.Insert(tableIndex, &IndexEntry{tableConnectCABuiltinSerial, next}); err != nil {
		return 0, fmt.Errorf("failed updating index: %s", err)
	}

	err = tx.Commit()
	return next, err
}
