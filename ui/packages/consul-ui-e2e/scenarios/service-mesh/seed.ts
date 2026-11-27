/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Ported from scripts/dev-service-mesh.sh, then extended to exercise every
 * conditional branch the Upstreams / Exposed Paths tables render.
 *
 * Registering Connect.SidecarService auto-registers "<name>-sidecar-proxy"
 * (Kind=connect-proxy, DestinationServiceID=<name>) — the trigger the UI looks
 * up via /proxy-instance to decide whether the Upstreams / Exposed Paths tabs
 * appear at all.
 *
 * Services registered and what each one covers:
 *   web          — Address branch (LocalBindPort) + fully-populated Expose.Paths
 *                  (Protocol / ListenerPort / LocalPathPort / Path all set).
 *   web-variants — socket-path branch (LocalBindSocketPath + LocalBindSocketMode)
 *                  and cross-DC branch (upstream Datacenter != dc1).
 *   web-tproxy   — sidecar Proxy.Mode="transparent" -> transparent-proxy warning
 *                  on the Upstreams tab.
 *   standalone   — plain service, NO sidecar -> neither tab appears (negative).
 */
export async function seed(consul: ConsulClient): Promise<void> {
  // Upstream targets so the Upstreams have real destinations.
  await consul.registerService({ Name: 'db', Port: 5432, Connect: { SidecarService: {} } });
  await consul.registerService({ Name: 'cache', Port: 6379, Connect: { SidecarService: {} } });

  // web — Address branch upstreams + full exposed paths.
  await consul.registerService({
    Name: 'web',
    Port: 8080,
    Connect: {
      SidecarService: {
        Proxy: {
          Upstreams: [
            { DestinationName: 'db', LocalBindPort: 9191 },
            { DestinationName: 'cache', LocalBindPort: 9192 },
          ],
          Expose: {
            Paths: [
              { Path: '/healthz', Protocol: 'http', LocalPathPort: 8080, ListenerPort: 21500 },
              { Path: '/metrics', Protocol: 'http', LocalPathPort: 8080, ListenerPort: 21501 },
            ],
          },
        },
      },
    },
  });

  // web-variants — socket-path branch + cross-DC branch.
  await consul.registerService({
    Name: 'web-variants',
    Port: 8081,
    Connect: {
      SidecarService: {
        Proxy: {
          Upstreams: [
            // socket-path branch: renders "Local bind socket path" + "Mode"
            // instead of an Address row. Consul rejects an upstream that sets
            // both a socket path and a bind port, so a socket-path upstream
            // never has a LocalBindPort to begin with.
            {
              DestinationName: 'db',
              LocalBindSocketPath: '/tmp/db.sock',
              LocalBindSocketMode: '0700',
            },
            // cross-DC branch: Datacenter != current dc (dc1) -> Datacenter row
            // renders; LocalBindPort set -> Address row renders too.
            { DestinationName: 'cache', LocalBindPort: 9291, Datacenter: 'dc2' },
          ],
        },
      },
    },
  });

  // web-tproxy — transparent proxy mode warning on the Upstreams tab.
  await consul.registerService({
    Name: 'web-tproxy',
    Port: 8082,
    Connect: {
      SidecarService: {
        Proxy: {
          Mode: 'transparent',
          Upstreams: [{ DestinationName: 'db', LocalBindPort: 9391 }],
        },
      },
    },
  });

  // standalone — no sidecar, so neither tab should appear.
  await consul.registerService({ Name: 'standalone', Port: 8083 });

  // Cheap assertions that the data landed before Playwright starts.
  for (const name of ['web', 'web-variants', 'web-tproxy']) {
    const sidecar = await consul.agentService(`${name}-sidecar-proxy`);
    if (sidecar.Kind !== 'connect-proxy') {
      throw new Error(`${name}-sidecar-proxy not registered as connect-proxy — tabs will not appear`);
    }
  }

  const tproxy = await consul.agentService('web-tproxy-sidecar-proxy');
  const mode = (tproxy.Proxy as { Mode?: string } | undefined)?.Mode;
  if (mode !== 'transparent') {
    throw new Error(`web-tproxy sidecar Proxy.Mode=${mode ?? '<unset>'} — expected "transparent"`);
  }
}
