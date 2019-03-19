package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

const (
	configTableName = "configurations"
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

// Configurations is used to pull all the configurations for the snapshot.
func (s *Snapshot) Configurations() ([]structs.Configuration, error) {
	ixns, err := s.tx.Get(configTableName, "id")
	if err != nil {
		return nil, err
	}

	var ret []structs.Configuration
	for wrapped := ixns.Next(); wrapped != nil; wrapped = ixns.Next() {
		ret = append(ret, wrapped.(structs.Configuration))
	}

	return ret, nil
}

// Configuration is used when restoring from a snapshot.
func (s *Restore) Configuration(c structs.Configuration) error {
	// Insert
	if err := s.tx.Insert(configTableName, c); err != nil {
		return fmt.Errorf("failed restoring configuration object: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, c.ModifyIndex, configTableName); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// EnsureConfiguration is called to upsert creation of a given configuration.
func (s *Store) EnsureConfiguration(idx uint64, conf structs.Configuration) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Does it make sense to validate here? We do this for service meta in the state store
	// but could also do this in RPC endpoint. More version compatibility that way?
	if err := conf.Validate(); err != nil {
		return fmt.Errorf("failed validating config: %v", err)
	}

	// Check for existing configuration.
	existing, err := tx.First("configurations", "id", conf.GetKind(), conf.GetName())
	if err != nil {
		return fmt.Errorf("failed configuration lookup: %s", err)
	}

	if existing != nil {
		conf.CreateIndex = serviceNode.CreateIndex
		conf.ModifyIndex = serviceNode.ModifyIndex
	} else {
		conf.CreateIndex = idx
	}
	conf.ModifyIndex = idx

	// Insert the configuration and update the index
	if err := tx.Insert("configurations", conf); err != nil {
		return fmt.Errorf("failed inserting service: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"configurations", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

// Configuration is called to get a given configuration.
func (s *Store) Configuration(idx uint64, kind structs.ConfigurationKind, name string) (structs.Configuration, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Get the existing configuration.
	existing, err := tx.First("configurations", "id", kind, name)
	if err != nil {
		return nil, fmt.Errorf("failed configuration lookup: %s", err)
	}

	conf, ok := existing.(structs.Configuration)
	if !ok {
		return nil, fmt.Errorf("configuration %q (%s) is an invalid type: %T", name, kind, conf)
	}

	return conf, nil
}
