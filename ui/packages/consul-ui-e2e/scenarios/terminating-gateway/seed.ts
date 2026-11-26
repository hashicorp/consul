/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Seeds the service detail page's "Linked Services" tab
 * (`app/templates/dc/services/show/services.hbs`), gated in
 * `app/templates/dc/services/show.hbs` to
 * `item.Service.Kind === 'terminating-gateway' && !item.Service.PeerName`.
 *
 * That tab's data comes from
 * `GET /v1/internal/ui/gateway-services-nodes/{gateway}?dc={dc}`
 * (`app/adapters/service.js`), which resolves the `terminating-gateway`
 * config entry named after the gateway, expands its `Services[]` (or a
 * `{Name: "*"}` wildcard, used here — the simplest way to link every backing
 * service in one shot) against the real catalog, and returns one row per
 * linked service name with instances/checks/tags/external-sources already
 * aggregated — exactly what `Consul::Service::List` renders. So this seeder
 * just needs: (1) the gateway service itself with `Kind: 'terminating-gateway'`,
 * (2) the wildcard config entry, and (3) a pile of ordinary backing services
 * with enough variety for real branch coverage in that table.
 *
 * Backing-service instances are registered as agentless catalog entries on
 * synthetic per-instance nodes (never on `node-primary`) — same reasoning as
 * `scenarios/nodes`: anything registered on the *live* agent's own node gets
 * reconciled by that agent's anti-entropy sync, which would silently
 * overwrite hand-picked Check statuses set via the raw catalog API.
 */
const GATEWAY_NAME = 'api-gateway';
const BACKEND_COUNT = 25;

const EXTERNAL_SOURCES = ['vault', 'kubernetes', 'terraform', 'consul-api-gateway', 'nomad'] as const;
const TAG_POOL = ['v1', 'v2', 'primary', 'canary', 'cache', 'http', 'grpc', 'internal', 'public', 'beta', 'edge', 'legacy'];
const HEALTH_CYCLE = ['passing', 'passing', 'passing', 'warning', 'critical'] as const;

interface BackendInstance {
  node: string;
  address: string;
  serviceId: string;
  port: number;
  status: (typeof HEALTH_CYCLE)[number];
}

interface Backend {
  name: string;
  tags: string[];
  meta?: Record<string, string>;
  instances: BackendInstance[];
}

function buildBackends(): Backend[] {
  const backends: Backend[] = [];

  for (let i = 1; i <= BACKEND_COUNT; i++) {
    const name = `backend-${String(i).padStart(2, '0')}`;

    // Instance count cycles 1 -> 2 -> 3 -> 1 -> ... so the instance-count
    // column has real spread, including single-instance services.
    const instanceCount = ((i - 1) % 3) + 1;

    // Every service is assigned a "target" health status; with >1 instance
    // only the last instance carries it (the rest are passing), so the
    // service's aggregated health still varies while most instances stay
    // healthy — mirrors a real fleet more than making every instance match.
    const targetStatus = HEALTH_CYCLE[(i - 1) % HEALTH_CYCLE.length];

    const instances: BackendInstance[] = [];
    for (let n = 1; n <= instanceCount; n++) {
      instances.push({
        node: `${name}-node-${n}`,
        address: `10.20.${i}.${n}`,
        serviceId: `${name}-${n}`,
        port: 9000 + n,
        status: n === instanceCount ? targetStatus : 'passing',
      });
    }

    // 5 services (every 5th, 1-based: 1, 6, 11, 16, 21) each get a distinct
    // external-source, so the "Registered via X" badge shows all 5 values.
    const meta = (i - 1) % 5 === 0 ? { 'external-source': EXTERNAL_SOURCES[Math.floor((i - 1) / 5) % EXTERNAL_SOURCES.length] } : undefined;

    // Leave two services untagged; give the rest 2-4 tags pulled
    // deterministically from the pool.
    const untagged = i === 3 || i === 17;
    const tagCount = 2 + (i % 3); // cycles 2, 3, 4
    const offset = (i * 2) % TAG_POOL.length;
    const tags = untagged
      ? []
      : Array.from({ length: tagCount }, (_, k) => TAG_POOL[(offset + k) % TAG_POOL.length]);

    backends.push({ name, tags, meta, instances });
  }

  return backends;
}

export async function seed(consul: ConsulClient): Promise<void> {
  const backends = buildBackends();

  for (const backend of backends) {
    for (const instance of backend.instances) {
      await consul.registerCatalogNode({
        Node: instance.node,
        Address: instance.address,
        Service: {
          Service: backend.name,
          ID: instance.serviceId,
          Port: instance.port,
          Tags: backend.tags,
          Meta: backend.meta,
        },
        Check: {
          Node: instance.node,
          CheckID: `service:${instance.serviceId}`,
          ServiceID: instance.serviceId,
          Name: `${backend.name} health`,
          Status: instance.status,
        },
      });
    }
  }

  // The terminating-gateway service itself — the tab only shows on a service
  // detail page whose Service.Kind is exactly 'terminating-gateway'.
  await consul.registerService({
    Kind: 'terminating-gateway',
    Name: GATEWAY_NAME,
    ID: GATEWAY_NAME,
    Port: 8443,
  });

  // The config entry that actually links the gateway to backing services.
  // A `{Name: "*"}` wildcard links every service in the catalog in one shot
  // — this is what drives every row in the Linked Services table.
  await consul.registerConfigEntry({
    Kind: 'terminating-gateway',
    Name: GATEWAY_NAME,
    Services: [{ Name: '*' }],
  });

  // Cheap assertion: hit the exact internal-UI endpoint the Linked Services
  // tab calls and confirm it resolved all the backing services, before
  // Playwright ever starts.
  const rows = await consul.gatewayServicesNodes(GATEWAY_NAME);
  if (rows.length !== BACKEND_COUNT) {
    throw new Error(
      `expected ${BACKEND_COUNT} linked services from gateway-services-nodes/${GATEWAY_NAME}, got ${rows.length}`,
    );
  }
}
