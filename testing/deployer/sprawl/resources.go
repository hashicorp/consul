// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"github.com/hashicorp/consul/testing/deployer/topology"
)

func (s *Sprawl) populateInitialResources(cluster *topology.Cluster) error {
	if len(cluster.InitialResources) == 0 {
		return nil
	}

	for _, res := range cluster.InitialResources {
		if _, err := s.writeResource(cluster, res); err != nil {
			return err
		}
	}

	return nil
}
