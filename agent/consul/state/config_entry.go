package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

const (
	configTableName = "config-entries"
)

// configTableSchema returns a new table schema used to store global service
// and proxy configurations.
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
		},
	}
}

func init() {
	registerSchema(configTableSchema)
}

// ConfigEntries is used to pull all the config entries for the snapshot.
func (s *Snapshot) ConfigEntries() ([]structs.ConfigEntry, error) {
	ixns, err := s.tx.Get(configTableName, "id")
	if err != nil {
		return nil, err
	}

	var ret []structs.ConfigEntry
	for wrapped := ixns.Next(); wrapped != nil; wrapped = ixns.Next() {
		ret = append(ret, wrapped.(structs.ConfigEntry))
	}

	return ret, nil
}

// Configuration is used when restoring from a snapshot.
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

// Configuration is called to get a given config entry.
func (s *Store) ConfigEntry(kind, name string) (uint64, structs.ConfigEntry, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Get the index
	idx := maxIndexTxn(tx, configTableName)

	// Get the existing config entry.
	existing, err := tx.First(configTableName, "id", kind, name)
	if err != nil {
		return 0, nil, fmt.Errorf("failed config entry lookup: %s", err)
	}
	if existing == nil {
		return 0, nil, nil
	}

	conf, ok := existing.(structs.ConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("config entry %q (%s) is an invalid type: %T", name, kind, conf)
	}

	return idx, conf, nil
}

// EnsureConfigEntry is called to upsert creation of a given config entry.
func (s *Store) EnsureConfigEntry(idx uint64, conf structs.ConfigEntry) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

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
	if err := tx.Insert("index", &IndexEntry{configTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

func (s *Store) DeleteConfigEntry(kind, name string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Get the index
	idx := maxIndexTxn(tx, configTableName)

	// Try to retrieve the existing health check.
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
