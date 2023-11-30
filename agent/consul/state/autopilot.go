// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

// autopilotConfigTableSchema returns a new table schema used for storing
// the autopilot configuration
func autopilotConfigTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "autopilot-config",
		Indexes: map[string]*memdb.IndexSchema{
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

// Autopilot is used to pull the autopilot config from the snapshot.
func (s *Snapshot) Autopilot() (*structs.AutopilotConfig, error) {
	c, err := s.tx.First("autopilot-config", "id")
	if err != nil {
		return nil, err
	}

	config, ok := c.(*structs.AutopilotConfig)
	if !ok {
		return nil, nil
	}

	return config, nil
}

// Autopilot is used when restoring from a snapshot.
func (s *Restore) Autopilot(config *structs.AutopilotConfig) error {
	if err := s.tx.Insert("autopilot-config", config); err != nil {
		return fmt.Errorf("failed restoring autopilot config: %s", err)
	}

	return nil
}

// AutopilotConfig is used to get the current Autopilot configuration.
func (s *Store) AutopilotConfig() (uint64, *structs.AutopilotConfig, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the autopilot config
	c, err := tx.First("autopilot-config", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed autopilot config lookup: %s", err)
	}

	config, ok := c.(*structs.AutopilotConfig)
	if !ok {
		return 0, nil, nil
	}

	return config.ModifyIndex, config, nil
}

// AutopilotSetConfig is used to set the current Autopilot configuration.
func (s *Store) AutopilotSetConfig(idx uint64, config *structs.AutopilotConfig) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := autopilotSetConfigTxn(tx, idx, config); err != nil {
		return err
	}

	return tx.Commit()
}

// AutopilotCASConfig is used to try updating the Autopilot configuration with a
// given Raft index. If the CAS index specified is not equal to the last observed index
// for the config, then the call is a noop,
func (s *Store) AutopilotCASConfig(idx, cidx uint64, config *structs.AutopilotConfig) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Check for an existing config
	existing, err := tx.First("autopilot-config", "id")
	if err != nil {
		return false, fmt.Errorf("failed autopilot config lookup: %s", err)
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	e, ok := existing.(*structs.AutopilotConfig)
	if !ok || e.ModifyIndex != cidx {
		return false, nil
	}

	if err := autopilotSetConfigTxn(tx, idx, config); err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

func autopilotSetConfigTxn(tx WriteTxn, idx uint64, config *structs.AutopilotConfig) error {
	// Check for an existing config
	existing, err := tx.First("autopilot-config", "id")
	if err != nil {
		return fmt.Errorf("failed autopilot config lookup: %s", err)
	}

	// Set the indexes.
	if existing != nil {
		config.CreateIndex = existing.(*structs.AutopilotConfig).CreateIndex
	} else {
		config.CreateIndex = idx
	}
	config.ModifyIndex = idx

	if err := tx.Insert("autopilot-config", config); err != nil {
		return fmt.Errorf("failed updating autopilot config: %s", err)
	}
	return nil
}
