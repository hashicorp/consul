/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Seeds the Nodes list page (HDS migration, see
 * consul-ui/docs/nodes-hds-migration.md) with catalog nodes that vary across
 * every column the redesign touches: Health (passing/warning/critical),
 * Address, and Services count. The live agent node (node-primary) is left as-is
 * to also cover the "agentful" case alongside these externally-registered
 * (agentless) nodes.
 */
type Health = 'passing' | 'warning' | 'critical';

interface NodeSpec {
  node: string;
  address: string;
  health: Health;
  services: string[];
}

const NODES: NodeSpec[] = [
  { node: 'app-01', address: '172.16.254.2', health: 'passing', services: ['web', 'api', 'redis'] },
  { node: 'app-02', address: '172.16.254.3', health: 'warning', services: ['worker'] },
  { node: 'db-01', address: '172.16.254.4', health: 'critical', services: ['postgres', 'pgbouncer', 'exporter', 'backup'] },
  { node: 'edge-01', address: '172.16.254.5', health: 'passing', services: [] },
  { node: 'cache-01', address: '172.16.254.6', health: 'passing', services: ['memcached', 'redis'] },
];

export async function seed(consul: ConsulClient): Promise<void> {
  for (const spec of NODES) {
    // Node-level serf health check carries the row's overall health status.
    await consul.registerCatalogNode({
      Node: spec.node,
      Address: spec.address,
      Check: {
        Node: spec.node,
        CheckID: 'serfHealth',
        Name: 'Serf Health Status',
        Status: spec.health,
      },
    });

    // Each service is a separate catalog register call against the same node.
    let port = 8000;
    for (const service of spec.services) {
      await consul.registerCatalogNode({
        Node: spec.node,
        Address: spec.address,
        Service: { Service: service, ID: `${service}-${spec.node}`, Port: port++ },
      });
    }
  }

  const total = (await consul.catalogNodes()).length;
  if (total < NODES.length) {
    throw new Error(`expected >= ${NODES.length} catalog nodes, found ${total}`);
  }
}
