// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sprawl

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/testing/deployer/topology"
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
			if ce.GetKind() == api.ServiceIntentions && strings.Contains(err.Error(), intentionsMigrationError) {
				logger.Warn("known error writing initial config entry; trying again",
					"kind", ce.GetKind(),
					"name", ce.GetName(),
					"namespace", ce.GetNamespace(),
					"partition", ce.GetPartition(),
					"error", err,
				)

				time.Sleep(500 * time.Millisecond)
				continue
			}
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

const intentionsMigrationError = `Intentions are read only while being upgraded to config entries`
