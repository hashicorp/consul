package state

import (
	"fmt"

	"github.com/hashicorp/consul/consul/structs"
)

// AutopilotConfig is used to get the current Autopilot configuration.
func (s *StateStore) AutopilotConfig() (*structs.AutopilotConfig, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the autopilot config
	c, err := tx.First("autopilot-config", "id")
	if err != nil {
		return nil, fmt.Errorf("failed autopilot config lookup: %s", err)
	}

	config, ok := c.(*structs.AutopilotConfig)
	if !ok {
		return nil, nil
	}

	return config, nil
}

// AutopilotConfig is used to set the current Autopilot configuration.
func (s *StateStore) UpdateAutopilotConfig(config *structs.AutopilotConfig) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := tx.Insert("autopilot-config", config); err != nil {
		return fmt.Errorf("failed updating autopilot config: %s", err)
	}

	tx.Commit()
	return nil
}
