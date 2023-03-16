package sprawl

import (
	"fmt"

	"github.com/hashicorp/consul-topology/topology"
)

func (s *Sprawl) populateInitialConfigEntries(cluster *topology.Cluster) error {
	if len(cluster.InitialConfigEntries) == 0 {
		return nil
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	for _, ce := range cluster.InitialConfigEntries {
		_, _, err := client.ConfigEntries().Set(ce, nil)
		if err != nil {
			return fmt.Errorf(
				"error persisting config entry kind=%q name=%q namespace=%q partition=%q: %w",
				ce.GetKind(),
				ce.GetName(),
				ce.GetNamespace(),
				ce.GetPartition(),
				err,
			)
		}
		logger.Info("wrote initial config entry",
			"kind", ce.GetKind(),
			"name", ce.GetName(),
			"namespace", ce.GetNamespace(),
			"partition", ce.GetPartition(),
		)
	}

	return nil
}
