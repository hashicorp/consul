package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

const federationStateTableName = "federation-states"

func federationStateTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: federationStateTableName,
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
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

func init() {
	registerSchema(federationStateTableSchema)
}

// FederationStates is used to pull all the federation states for the snapshot.
func (s *Snapshot) FederationStates() ([]*structs.FederationState, error) {
	configs, err := s.tx.Get(federationStateTableName, "id")
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
	if err := s.tx.Insert(federationStateTableName, g); err != nil {
		return fmt.Errorf("failed restoring federation state object: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, g.ModifyIndex, federationStateTableName); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func (s *Store) FederationStateBatchSet(idx uint64, configs structs.FederationStates) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, config := range configs {
		if err := s.federationStateSetTxn(tx, idx, config); err != nil {
			return err
		}
	}

	tx.Commit()
	return nil
}

// FederationStateSet is called to do an upsert of a given federation state.
func (s *Store) FederationStateSet(idx uint64, config *structs.FederationState) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.federationStateSetTxn(tx, idx, config); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// federationStateSetTxn upserts a federation state inside of a transaction.
func (s *Store) federationStateSetTxn(tx *memdb.Txn, idx uint64, config *structs.FederationState) error {
	if config.Datacenter == "" {
		return fmt.Errorf("missing datacenter on federation state")
	}

	// Check for existing.
	var existing *structs.FederationState
	existingRaw, err := tx.First(federationStateTableName, "id", config.Datacenter)
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
	if err := tx.Insert(federationStateTableName, config); err != nil {
		return fmt.Errorf("failed inserting federation state: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{federationStateTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	return nil
}

// FederationStateGet is called to get a federation state.
func (s *Store) FederationStateGet(ws memdb.WatchSet, datacenter string) (uint64, *structs.FederationState, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.federationStateGetTxn(tx, ws, datacenter)
}

func (s *Store) federationStateGetTxn(tx *memdb.Txn, ws memdb.WatchSet, datacenter string) (uint64, *structs.FederationState, error) {
	// Get the index
	idx := maxIndexTxn(tx, federationStateTableName)

	// Get the existing contents.
	watchCh, existing, err := tx.FirstWatch(federationStateTableName, "id", datacenter)
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
	return s.federationStateListTxn(tx, ws)
}

func (s *Store) federationStateListTxn(tx *memdb.Txn, ws memdb.WatchSet) (uint64, []*structs.FederationState, error) {
	// Get the index
	idx := maxIndexTxn(tx, federationStateTableName)

	iter, err := tx.Get(federationStateTableName, "id")
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
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.federationStateDeleteTxn(tx, idx, datacenter); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) FederationStateBatchDelete(idx uint64, datacenters []string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	for _, datacenter := range datacenters {
		if err := s.federationStateDeleteTxn(tx, idx, datacenter); err != nil {
			return err
		}
	}

	tx.Commit()
	return nil
}

func (s *Store) federationStateDeleteTxn(tx *memdb.Txn, idx uint64, datacenter string) error {
	// Try to retrieve the existing federation state.
	existing, err := tx.First(federationStateTableName, "id", datacenter)
	if err != nil {
		return fmt.Errorf("failed federation state lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	// Delete the federation state from the DB and update the index.
	if err := tx.Delete(federationStateTableName, existing); err != nil {
		return fmt.Errorf("failed removing federation state: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{federationStateTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}
