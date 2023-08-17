// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tfgen

import (
	"fmt"
)

// digestOutputs takes the data extracted from terraform output variables and
// updates various fields on the topology.Topology with that data.
func (g *Generator) digestOutputs(out *Outputs) error {
	for clusterName, nodeMap := range out.Nodes {
		cluster, ok := g.topology.Clusters[clusterName]
		if !ok {
			return fmt.Errorf("found output cluster that does not exist: %s", clusterName)
		}
		for nid, nodeOut := range nodeMap {
			node := cluster.NodeByID(nid)
			if node == nil {
				return fmt.Errorf("found output node that does not exist in cluster %q: %s", nid, clusterName)
			}

			if node.DigestExposedPorts(nodeOut.Ports) {
				g.logger.Info("discovered exposed port mappings",
					"cluster", clusterName,
					"node", nid.String(),
					"ports", nodeOut.Ports,
				)
			}
		}
	}

	for netName, proxyPort := range out.ProxyPorts {
		changed, err := g.topology.DigestExposedProxyPort(netName, proxyPort)
		if err != nil {
			return err
		}
		if changed {
			g.logger.Info("discovered exposed forward proxy port",
				"network", netName,
				"port", proxyPort,
			)
		}
	}

	return nil
}
