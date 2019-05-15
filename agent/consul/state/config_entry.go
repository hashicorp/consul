package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

const (
	configTableName = "config-entries"
)

// configTableSchema returns a new table schema used to store global
// config entries.
func configTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: configTableName,
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Kind",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "Name",
							Lowercase: true,
						},
					},
				},
			},
			"kind": &memdb.IndexSchema{
				Name:         "kind",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Kind",
					Lowercase: true,
				},
			},
		},
	}
}

func init() {
	registerSchema(configTableSchema)
}

// ConfigEntries is used to pull all the config entries for the snapshot.
func (s *Snapshot) ConfigEntries() ([]structs.ConfigEntry, error) {
	entries, err := s.tx.Get(configTableName, "id")
	if err != nil {
		return nil, err
	}

	var ret []structs.ConfigEntry
	for wrapped := entries.Next(); wrapped != nil; wrapped = entries.Next() {
		ret = append(ret, wrapped.(structs.ConfigEntry))
	}

	return ret, nil
}

// ConfigEntry is used when restoring from a snapshot.
func (s *Restore) ConfigEntry(c structs.ConfigEntry) error {
	// Insert
	if err := s.tx.Insert(configTableName, c); err != nil {
		return fmt.Errorf("failed restoring config entry object: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, c.GetRaftIndex().ModifyIndex, configTableName); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// ConfigEntry is called to get a given config entry.
func (s *Store) ConfigEntry(ws memdb.WatchSet, kind, name string) (uint64, structs.ConfigEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.configEntryTxn(tx, ws, kind, name)
}

func (s *Store) configEntryTxn(tx *memdb.Txn, ws memdb.WatchSet, kind, name string) (uint64, structs.ConfigEntry, error) {
	// Get the index
	idx := maxIndexTxn(tx, configTableName)

	// Get the existing config entry.
	watchCh, existing, err := tx.FirstWatch(configTableName, "id", kind, name)
	if err != nil {
		return 0, nil, fmt.Errorf("failed config entry lookup: %s", err)
	}
	ws.Add(watchCh)
	if existing == nil {
		return idx, nil, nil
	}

	conf, ok := existing.(structs.ConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("config entry %q (%s) is an invalid type: %T", name, kind, conf)
	}

	return idx, conf, nil
}

// ConfigEntries is called to get all config entry objects.
func (s *Store) ConfigEntries(ws memdb.WatchSet) (uint64, []structs.ConfigEntry, error) {
	return s.ConfigEntriesByKind(ws, "")
}

// ConfigEntriesByKind is called to get all config entry objects with the given kind.
// If kind is empty, all config entries will be returned.
func (s *Store) ConfigEntriesByKind(ws memdb.WatchSet, kind string) (uint64, []structs.ConfigEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the index
	idx := maxIndexTxn(tx, configTableName)

	// Lookup by kind, or all if kind is empty
	var iter memdb.ResultIterator
	var err error
	if kind != "" {
		iter, err = tx.Get(configTableName, "kind", kind)
	} else {
		iter, err = tx.Get(configTableName, "id")
	}
	if err != nil {
		return 0, nil, fmt.Errorf("failed config entry lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results []structs.ConfigEntry
	for v := iter.Next(); v != nil; v = iter.Next() {
		results = append(results, v.(structs.ConfigEntry))
	}
	return idx, results, nil
}

// EnsureConfigEntry is called to do an upsert of a given config entry.
func (s *Store) EnsureConfigEntry(idx uint64, conf structs.ConfigEntry) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.ensureConfigEntryTxn(tx, idx, conf); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// ensureConfigEntryTxn upserts a config entry inside of a transaction.
func (s *Store) ensureConfigEntryTxn(tx *memdb.Txn, idx uint64, conf structs.ConfigEntry) error {
	// Check for existing configuration.
	existing, err := tx.First(configTableName, "id", conf.GetKind(), conf.GetName())
	if err != nil {
		return fmt.Errorf("failed configuration lookup: %s", err)
	}

	raftIndex := conf.GetRaftIndex()
	if existing != nil {
		existingIdx := existing.(structs.ConfigEntry).GetRaftIndex()
		raftIndex.CreateIndex = existingIdx.CreateIndex
		raftIndex.ModifyIndex = existingIdx.ModifyIndex
	} else {
		raftIndex.CreateIndex = idx
	}
	raftIndex.ModifyIndex = idx

	// Insert the config entry and update the index
	if err := tx.Insert(configTableName, conf); err != nil {
		return fmt.Errorf("failed inserting config entry: %s", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, configTableName); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	return nil
}

// EnsureConfigEntryCAS is called to do a check-and-set upsert of a given config entry.
func (s *Store) EnsureConfigEntryCAS(idx, cidx uint64, conf structs.ConfigEntry) (bool, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Check for existing configuration.
	existing, err := tx.First(configTableName, "id", conf.GetKind(), conf.GetName())
	if err != nil {
		return false, fmt.Errorf("failed configuration lookup: %s", err)
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	var existingIdx structs.RaftIndex
	if existing != nil {
		existingIdx = *existing.(structs.ConfigEntry).GetRaftIndex()
	}
	if cidx == 0 && existing != nil {
		return false, nil
	}
	if cidx != 0 && existing == nil {
		return false, nil
	}
	if existing != nil && cidx != 0 && cidx != existingIdx.ModifyIndex {
		return false, nil
	}

	if err := s.ensureConfigEntryTxn(tx, idx, conf); err != nil {
		return false, err
	}

	tx.Commit()
	return true, nil
}

func (s *Store) DeleteConfigEntry(idx uint64, kind, name string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Try to retrieve the existing config entry.
	existing, err := tx.First(configTableName, "id", kind, name)
	if err != nil {
		return fmt.Errorf("failed config entry lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	// Delete the config entry from the DB and update the index.
	if err := tx.Delete(configTableName, existing); err != nil {
		return fmt.Errorf("failed removing check: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{configTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}
