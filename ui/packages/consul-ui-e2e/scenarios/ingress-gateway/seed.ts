/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Seeds the service detail page's "Upstreams" tab
 * (`app/templates/dc/services/show/upstreams.hbs`), gated in
 * `app/templates/dc/services/show.hbs` to
 * `item.Service.Kind === 'ingress-gateway' && !item.Service.PeerName`.
 *
 * That tab shares the *exact same* data source as terminating-gateway's
 * "Linked Services" tab: `GET /v1/internal/ui/gateway-services-nodes/{gateway}
 * ?dc={dc}` (`app/adapters/service.js`). The Go handler
 * (`UIGatewayServicesNodes` -> `Internal.GatewayServiceDump`) resolves
 * whichever gateway config entry is named after the gateway (terminating- or
 * ingress-), expands its linked services against the real catalog, and
 * returns one aggregated row per service name.
 *
 * The row shape is identical either way (`ServiceSummary`), but *how it gets
 * populated* differs, and so does the component that renders it:
 *
 * - `terminating-gateway` config entries have a flat `Services: [{Name}]`
 *   list (or a `{Name: "*"}` wildcard) with no port per service — so
 *   `GatewayConfig.Addresses` never gets populated for that tab, and
 *   `Consul::Service::Table` (used there) never reads it.
 * - `ingress-gateway` config entries instead have `Listeners: [{Port,
 *   Protocol, Services: [{Name}]}]` — each *listener* declares the port
 *   traffic arrives on for its linked services. The server turns that into a
 *   real `host:port` address per linked service (`GatewayService.Addresses`
 *   in `agent/structs/config_entry_gateways.go`, called from
 *   `modifySummaryForGatewayService` in `agent/ui_endpoint.go`), defaulting to
 *   a DNS name of the form `<service>.ingress.<dc>.<domain>` (here, the
 *   agent's default domain "consul.") when the listener has no explicit
 *   `Hosts`. That's exactly what `Consul::Upstream::List`
 *   (`app/components/consul/upstream/list/index.hbs`) renders per row under
 *   an "Address" field with a copy button — one row can have *several*
 *   addresses if the same service is linked from more than one listener.
 *
 * `Consul::Upstream::List` is also visually much simpler than
 * `Consul::Service::Table`/`Consul::Service::List`: it's a card list (not a
 * data table), and it renders only:
 *   - the service name (linked, only if `InstanceCount > 0`; otherwise a
 *     plain, unlinked `<p>`)
 *   - a health indicator derived from `item.MeshStatus` (only rendered
 *     alongside the *linked* name — i.e. only when `InstanceCount > 0`; an
 *     unregistered upstream shows neither a link nor a health indicator)
 *   - `Consul::Bucket::List` (namespace/partition/peer pills — hidden in OSS
 *     with default ns/partition, same as everywhere else)
 *   - one "Address" row per `item.GatewayConfig.Addresses` entry
 * It does NOT render tags, the "Registered via X" external-source badge, an
 * instance-count number, or the service Kind — all present on
 * `Consul::Service::Table` but simply absent from this template. So, unlike
 * `scenarios/terminating-gateway`, this seeder does not bother varying tags
 * or `external-source` meta: nothing in this tab would ever show them.
 *
 * What this seeder *does* vary, because the component actually renders it:
 *   - instance count: 0 (never registered), 1, 2, 3
 *   - aggregated health: passing, warning, critical (via one "off" instance
 *     health check per multi-instance service, same technique as
 *     terminating-gateway)
 *   - address count per row: most services are on a single listener (one
 *     address); one is deliberately linked from two listeners with two
 *     different ports, producing two addresses on the same row
 *
 * Quirk worth calling out explicitly for whoever writes the Playwright specs:
 * `Internal.GatewayServiceDump` (`agent/consul/internal_endpoint.go`) fetches
 * catalog instances *once per matching `GatewayService` mapping* (i.e. once
 * per listener a service is linked from), not once per service overall.
 * `summarizeServices` (`agent/ui_endpoint.go`) then does an unconditional
 * `sum.InstanceCount += 1` per dump row with no de-duplication by node/service
 * ID (unlike `Tags`/`ExternalSources`/`Nodes`, which dedupe explicitly).
 * Checks *do* dedupe (keyed by node+check ID), so aggregated health stays
 * correct — but a service linked from N listeners reports `InstanceCount`
 * inflated by a factor of N. Confirmed against a live agent: `upstream-04`
 * has exactly **one** registered catalog instance but is linked from **two**
 * listeners, and `GET /v1/internal/ui/gateway-services-nodes/ingress-gateway`
 * reports `"InstanceCount": 2` for it (see the assertion below) — the same
 * number as `upstream-02`'s two genuinely-separate instances. This is real,
 * deterministic backend behavior (not a seeding bug), so the test spec
 * should assert `InstanceCount` for `upstream-04` is `2`, not `1`.
 *
 * Backing-service instances are registered as agentless catalog entries on
 * synthetic per-instance nodes (never on `node-primary`) — same reasoning as
 * `scenarios/terminating-gateway`/`scenarios/nodes`: anything registered on
 * the *live* agent's own node gets reconciled by that agent's anti-entropy
 * sync, which would silently overwrite hand-picked Check statuses set via the
 * raw catalog API.
 */
const GATEWAY_NAME = 'ingress-gateway';

interface BackendInstance {
  node: string;
  address: string;
  serviceId: string;
  port: number;
  status: 'passing' | 'warning' | 'critical';
}

interface Backend {
  name: string;
  instances: BackendInstance[];
}

// upstream-01: single instance, all-passing -> a plain healthy row, one
// address, and the simplest possible "everything's fine" case.
// upstream-02: two instances, the second critical -> aggregated MeshStatus
// "critical" while most instances still pass.
// upstream-03: three instances, the third warning -> aggregated MeshStatus
// "warning".
// upstream-04: one real catalog instance, passing, but linked from *two*
// listeners (see LISTENERS below) -> two Addresses on one row, and (per the
// backend quirk documented above) a reported InstanceCount of 2, not 1.
// upstream-05: no catalog instances at all -> InstanceCount 0; the row shows
// an unlinked plain-text name and no health indicator, but still gets an
// Address, since the listener/port is defined regardless of whether the
// upstream is actually registered (mirrors the real
// `TestInternal_GatewayServiceDump_Ingress` server test case for an
// unregistered "web" upstream).
const BACKENDS: Backend[] = [
  {
    name: 'upstream-01',
    instances: [{ node: 'upstream-01-node-1', address: '10.30.1.1', serviceId: 'upstream-01-1', port: 9101, status: 'passing' }],
  },
  {
    name: 'upstream-02',
    instances: [
      { node: 'upstream-02-node-1', address: '10.30.2.1', serviceId: 'upstream-02-1', port: 9201, status: 'passing' },
      { node: 'upstream-02-node-2', address: '10.30.2.2', serviceId: 'upstream-02-2', port: 9202, status: 'critical' },
    ],
  },
  {
    name: 'upstream-03',
    instances: [
      { node: 'upstream-03-node-1', address: '10.30.3.1', serviceId: 'upstream-03-1', port: 9301, status: 'passing' },
      { node: 'upstream-03-node-2', address: '10.30.3.2', serviceId: 'upstream-03-2', port: 9302, status: 'passing' },
      { node: 'upstream-03-node-3', address: '10.30.3.3', serviceId: 'upstream-03-3', port: 9303, status: 'warning' },
    ],
  },
  {
    name: 'upstream-04',
    instances: [{ node: 'upstream-04-node-1', address: '10.30.4.1', serviceId: 'upstream-04-1', port: 9401, status: 'passing' }],
  },
  {
    // Deliberately no instances registered — stays a GatewayService-only row.
    name: 'upstream-05',
    instances: [],
  },
];

// One TCP listener per linked service (TCP listeners only support a single
// service each; only HTTP-family listeners support multiplexing several
// services on one port — see `IngressGatewayConfigEntry.Validate` in
// `agent/structs/config_entry_gateways.go`). `upstream-04` is deliberately
// declared on two listeners/ports, so its row gets two Addresses.
const LISTENERS = [
  { Port: 8081, Protocol: 'tcp', Services: [{ Name: 'upstream-01' }] },
  { Port: 8082, Protocol: 'tcp', Services: [{ Name: 'upstream-02' }] },
  { Port: 8083, Protocol: 'tcp', Services: [{ Name: 'upstream-03' }] },
  { Port: 8084, Protocol: 'tcp', Services: [{ Name: 'upstream-04' }] },
  { Port: 8090, Protocol: 'tcp', Services: [{ Name: 'upstream-04' }] },
  { Port: 8085, Protocol: 'tcp', Services: [{ Name: 'upstream-05' }] },
];

export async function seed(consul: ConsulClient): Promise<void> {
  for (const backend of BACKENDS) {
    for (const instance of backend.instances) {
      await consul.registerCatalogNode({
        Node: instance.node,
        Address: instance.address,
        Service: {
          Service: backend.name,
          ID: instance.serviceId,
          Port: instance.port,
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

  // The ingress-gateway service itself — the Upstreams tab only shows on a
  // service detail page whose Service.Kind is exactly 'ingress-gateway'.
  await consul.registerService({
    Kind: 'ingress-gateway',
    Name: GATEWAY_NAME,
    ID: GATEWAY_NAME,
    Port: 8443,
  });

  // The config entry that actually links the gateway to upstream services.
  // Unlike terminating-gateway's flat `Services[]`, ingress-gateway groups
  // services under per-port `Listeners[]` — this is what drives both the
  // rows *and* each row's Address(es) in the Upstreams table.
  await consul.registerConfigEntry({
    Kind: 'ingress-gateway',
    Name: GATEWAY_NAME,
    Listeners: LISTENERS,
  });

  // Cheap assertion: hit the exact internal-UI endpoint the Upstreams tab
  // calls and confirm it resolved every linked service (including the
  // never-registered one) with the expected instance counts and address
  // counts, before Playwright ever starts.
  const rows = await consul.gatewayServicesNodes(GATEWAY_NAME);
  if (rows.length !== BACKENDS.length) {
    throw new Error(`expected ${BACKENDS.length} linked services from gateway-services-nodes/${GATEWAY_NAME}, got ${rows.length}`);
  }

  const byName = new Map(rows.map((row) => [row.Name as string, row]));

  const upstream04 = byName.get('upstream-04');
  const addresses04 = (upstream04?.GatewayConfig as { Addresses?: string[] } | undefined)?.Addresses ?? [];
  if (addresses04.length !== 2) {
    throw new Error(`expected upstream-04 to have 2 addresses (linked from 2 listeners), got ${addresses04.length}: ${JSON.stringify(addresses04)}`);
  }

  // upstream-04 has exactly 1 real catalog instance, but the backend counts
  // InstanceCount once per matching GatewayService (i.e. once per listener
  // it's linked from) with no de-duplication — see the doc comment above.
  // Linked from 2 listeners here, so the reported InstanceCount is 2.
  if ((upstream04?.InstanceCount as number | undefined) !== 2) {
    throw new Error(`expected upstream-04 (1 real instance, linked from 2 listeners) to report InstanceCount 2, got ${JSON.stringify(upstream04?.InstanceCount)}`);
  }

  const upstream05 = byName.get('upstream-05');
  if ((upstream05?.InstanceCount as number | undefined) !== 0) {
    throw new Error(`expected upstream-05 (never registered) to have InstanceCount 0, got ${JSON.stringify(upstream05?.InstanceCount)}`);
  }
  const addresses05 = (upstream05?.GatewayConfig as { Addresses?: string[] } | undefined)?.Addresses ?? [];
  if (addresses05.length !== 1) {
    throw new Error(`expected upstream-05 to still have 1 address from its listener, got ${addresses05.length}`);
  }
}
