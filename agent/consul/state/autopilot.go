package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/go-memdb"
)

// autopilotConfigTableSchema returns a new table schema used for storing
// the autopilot configuration
func autopilotConfigTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "autopilot-config",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
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

func init() {
	registerSchema(autopilotConfigTableSchema)
}

// Autopilot is used to pull the autopilot config from the snapshot.
func (s *Snapshot) Autopilot() (*autopilot.Config, error) {
	c, err := s.tx.First("autopilot-config", "id")
	if err != nil {
		return nil, err
	}

	config, ok := c.(*autopilot.Config)
	if !ok {
		return nil, nil
	}

	return config, nil
}

// Autopilot is used when restoring from a snapshot.
func (s *Restore) Autopilot(config *autopilot.Config) error {
	if err := s.tx.Insert("autopilot-config", config); err != nil {
		return fmt.Errorf("failed restoring autopilot config: %s", err)
	}

	return nil
}

// AutopilotConfig is used to get the current Autopilot configuration.
func (s *Store) AutopilotConfig() (uint64, *autopilot.Config, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the autopilot config
	c, err := tx.First("autopilot-config", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed autopilot config lookup: %s", err)
	}

	config, ok := c.(*autopilot.Config)
	if !ok {
		return 0, nil, nil
	}

	return config.ModifyIndex, config, nil
}

// AutopilotSetConfig is used to set the current Autopilot configuration.
func (s *Store) AutopilotSetConfig(idx uint64, config *autopilot.Config) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	s.autopilotSetConfigTxn(idx, tx, config)

	tx.Commit()
	return nil
}

// AutopilotCASConfig is used to try updating the Autopilot configuration with a
// given Raft index. If the CAS index specified is not equal to the last observed index
// for the config, then the call is a noop,
func (s *Store) AutopilotCASConfig(idx, cidx uint64, config *autopilot.Config) (bool, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Check for an existing config
	existing, err := tx.First("autopilot-config", "id")
	if err != nil {
		return false, fmt.Errorf("failed autopilot config lookup: %s", err)
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	e, ok := existing.(*autopilot.Config)
	if !ok || e.ModifyIndex != cidx {
		return false, nil
	}

	s.autopilotSetConfigTxn(idx, tx, config)

	tx.Commit()
	return true, nil
}

func (s *Store) autopilotSetConfigTxn(idx uint64, tx *memdb.Txn, config *autopilot.Config) error {
	// Check for an existing config
	existing, err := tx.First("autopilot-config", "id")
	if err != nil {
		return fmt.Errorf("failed autopilot config lookup: %s", err)
	}

	// Set the indexes.
	if existing != nil {
		config.CreateIndex = existing.(*autopilot.Config).CreateIndex
	} else {
		config.CreateIndex = idx
	}
	config.ModifyIndex = idx

	if err := tx.Insert("autopilot-config", config); err != nil {
		return fmt.Errorf("failed updating autopilot config: %s", err)
	}
	return nil
}
