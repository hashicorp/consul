/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Ported from a manual live-testing pass over the Intentions UI (create/edit
 * form, list, and the Layer 7 permission builder) against a real dev agent.
 *
 * Registers the plain (non-Connect) services intentions reference, then three
 * intentions covering every `Action` branch the list/table and form render:
 *
 *   web -> api        Allow
 *   *   -> payments   Deny            (wildcard source -> "All Services (*)")
 *   web -> payments   Application Aware, one Layer 7 permission:
 *                       Deny, PathPrefix "/admin", Methods [GET, POST],
 *                       Header X-Debug exactly "true", Header X-Trace-Id present
 *
 * `payments` gets a `service-defaults` config entry setting `Protocol: "http"`
 * *before* the L7 intention is registered — Consul rejects Layer 7 permissions
 * against a plain TCP destination with a 500
 * (`has protocol "tcp", which is incompatible with L7 intentions permissions`),
 * which only surfaces after clicking Save in the UI with no client-side hint.
 * Setting the protocol here means the scenario seeds cleanly and the 500 never
 * fires — see the seed as documentation of that requirement, not just data.
 */
export async function seed(consul: ConsulClient): Promise<void> {
  // Plain catalog services. Intentions don't need Connect sidecars — only the
  // destination service needs an HTTP-compatible protocol for L7 permissions.
  for (const [name, port] of [
    ['web', 9090],
    ['api', 9091],
    ['payments', 9092],
    ['checkout', 9093],
  ] as const) {
    await consul.registerService({ Name: name, Port: port });
  }

  // Required before `payments`/`checkout` can carry a Layer 7 intention
  // permission — `checkout` is otherwise free (no seeded intention), so tests
  // use it as an L7 create target too.
  await consul.registerConfigEntry({ Kind: 'service-defaults', Name: 'payments', Protocol: 'http' });
  await consul.registerConfigEntry({ Kind: 'service-defaults', Name: 'checkout', Protocol: 'http' });

  await consul.registerIntention({ SourceName: 'web', DestinationName: 'api', Action: 'allow' });

  await consul.registerIntention({ SourceName: '*', DestinationName: 'payments', Action: 'deny' });

  await consul.registerIntention({
    SourceName: 'web',
    DestinationName: 'payments',
    Permissions: [
      {
        Action: 'deny',
        HTTP: {
          PathPrefix: '/admin',
          Methods: ['GET', 'POST'],
          Header: [
            { Name: 'X-Debug', Exact: 'true' },
            { Name: 'X-Trace-Id', Present: true },
          ],
        },
      },
    ],
  });

  // Cheap assertion that the data landed before Playwright starts.
  const seeded = await consul.intentions();
  if (seeded.length < 3) {
    throw new Error(`expected 3 seeded intentions, found ${seeded.length}`);
  }
  const l7 = seeded.find(
    (i) => i.SourceName === 'web' && i.DestinationName === 'payments',
  ) as { Permissions?: unknown[] } | undefined;
  if (!l7?.Permissions?.length) {
    throw new Error('web -> payments intention missing its Layer 7 Permissions — L7 seed failed');
  }
}
