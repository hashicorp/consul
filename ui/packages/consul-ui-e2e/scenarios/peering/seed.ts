/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { ConsulClient } from '../../lib/consul-client.ts';
import { waitFor } from '../../lib/wait.ts';

/**
 * Seeds `/ui/dc1/peers` — cluster peering. Unlike every other scenario, this
 * one is TWO independent Consul clusters, not one agent:
 *
 *   - the primary agent on the fixed :8500 (node-primary) — what
 *     Playwright/ember actually talks to, and the `consul` argument `seed()`
 *     receives (same as any other scenario).
 *   - a second, wholly separate cluster on :8510 (node-peer) — started
 *     alongside the primary by `local.sh`/`docker-compose.yml`, but only
 *     ever talked to directly by this file, via a second `ConsulClient`
 *     constructed explicitly below. consul-ui itself never talks to it.
 *
 * `billing-cluster` is deliberately set up so ALL THREE of consul-ui's peer
 * detail tabs have real data — Imported Services, Exported Services, AND
 * Server Addresses:
 *
 *   - Imported/Exported Services just need config entries on either side —
 *     which side dialed doesn't matter, the exchange is bidirectional once
 *     the peering stream is up.
 *   - Server Addresses is the one that DOES care about direction:
 *     consul-ui only renders that tab when the *local* peering record has a
 *     `PeerID` set (`Peer#isReceiver` in consul-ui's `models/peer.js` — the
 *     comment there: "only the side that establishes will hold this
 *     property"). So the PRIMARY has to be the one that calls
 *     `/v1/peering/establish` (the dialer/"receiver of the token"), not the
 *     one that generates it — the reverse of the naive "acceptor generates,
 *     dialer establishes" reading of the docs. Confirmed against a real
 *     agent: generate on the PEER, establish on the PRIMARY, and only then
 *     does the primary's own `/v1/peering/billing-cluster` come back with a
 *     non-empty `PeerServerAddresses` + a `PeerID`.
 *
 * The flow:
 *
 *   1. Register a real Connect service (`payments`, with a sidecar) on the
 *      PEER cluster, and export it to the primary — so Imported Services
 *      has something.
 *   2. Register a real Connect service (`invoices`, with a sidecar) on the
 *      PRIMARY, and export it to the peer — so Exported Services has
 *      something.
 *   3. Generate a peering token on the PEER (the acceptor here) for a peer
 *      it calls `primary-cluster`.
 *   4. Establish that token from the PRIMARY (the dialer here), naming the
 *      peer `billing-cluster` — this is the name that ends up in the
 *      primary's `/v1/peerings` and consul-ui's peers list/detail route
 *      (`/ui/dc1/peers/billing-cluster`).
 *   5. Poll `peerings()` on the primary until `billing-cluster` is
 *      `"ACTIVE"` — this is the seeder's cheap end-of-seed assertion.
 *
 * Also generates a SECOND token, `reporting-cluster` — the primary
 * generates it (as an acceptor would) and nothing ever consumes it — so the
 * peers list shows both an `ACTIVE` and a `PENDING` status badge. Since
 * nothing establishes it, its record never gets a `PeerID`/addresses, which
 * is fine: it's only there for list-page badge variety, not a detail-page
 * subject.
 */

/** Must match docker-compose.yml/local.sh's second agent (node-peer). */
const PEER_ADDR = 'http://127.0.0.1:8510';

/** What the primary calls the peer — also the route segment: `/ui/dc1/peers/billing-cluster`. */
const PRIMARY_SIDE_PEER_NAME = 'billing-cluster';
/** What the peer calls the primary. */
const PEER_SIDE_PEER_NAME = 'primary-cluster';
/** A second peer, deliberately never established — stays PENDING. */
const PENDING_PEER_NAME = 'reporting-cluster';

const IMPORTED_SERVICE = 'payments'; // peer -> primary
const EXPORTED_SERVICE = 'invoices'; // primary -> peer

export async function seed(consul: ConsulClient): Promise<void> {
  const peer = new ConsulClient(PEER_ADDR);
  await peer.waitReady();
  await peer.waitConnectCA();

  // ── 1. Imported side: real Connect service on the peer, exported to the primary ──
  await peer.registerService({
    Name: IMPORTED_SERVICE,
    Port: 8080,
    Connect: { SidecarService: {} },
  });
  await peer.registerConfigEntry({
    Kind: 'exported-services',
    Name: 'default',
    Services: [
      {
        Name: IMPORTED_SERVICE,
        Namespace: 'default',
        Consumers: [{ Peer: PEER_SIDE_PEER_NAME }],
      },
    ],
  });

  // ── 2. Exported side: real Connect service on the primary, exported to the peer ──
  await consul.registerService({
    Name: EXPORTED_SERVICE,
    Port: 8081,
    Connect: { SidecarService: {} },
  });
  await consul.registerConfigEntry({
    Kind: 'exported-services',
    Name: 'default',
    Services: [
      {
        Name: EXPORTED_SERVICE,
        Namespace: 'default',
        Consumers: [{ Peer: PRIMARY_SIDE_PEER_NAME }],
      },
    ],
  });

  // ── 3. Generate a token on the PEER (acceptor here — reversed from the
  // scenario's earlier draft, see the doc comment above on why) ───────────
  const { PeeringToken } = await peer.generatePeeringToken({ PeerName: PEER_SIDE_PEER_NAME });
  if (!PeeringToken) throw new Error('generatePeeringToken() did not return a PeeringToken');

  // ── 4. Establish it from the PRIMARY (dialer here) ──────────────────────
  await consul.establishPeering({ PeerName: PRIMARY_SIDE_PEER_NAME, PeeringToken });

  // ── 5. Poll until ACTIVE — the cheap assertion this scenario's seed lives
  // or dies by ─────────────────────────────────────────────────────────────
  await waitFor(
    async () => {
      const p = await consul.peering(PRIMARY_SIDE_PEER_NAME);
      return p.State === 'ACTIVE';
    },
    { timeoutMs: 30_000, label: `peering "${PRIMARY_SIDE_PEER_NAME}" to reach ACTIVE` },
  );
  const active = (await consul.peering(PRIMARY_SIDE_PEER_NAME)) as {
    ID?: string;
    PeerID?: string;
    PeerServerAddresses?: string[];
  };
  console.log(`  Peering "${PRIMARY_SIDE_PEER_NAME}" is ACTIVE (ID: ${active.ID})`);

  // Hard assertion: the primary must be the *establishing* side, or the
  // Server Addresses tab never renders (see doc comment above).
  if (!active.PeerID) {
    throw new Error(
      `Peering "${PRIMARY_SIDE_PEER_NAME}" has no PeerID — the primary must be the establishing side for` +
        ` consul-ui's Server Addresses tab (isReceiver in models/peer.js) to render. Check which side called` +
        ` establishPeering().`,
    );
  }
  if (!active.PeerServerAddresses?.length) {
    throw new Error(`Peering "${PRIMARY_SIDE_PEER_NAME}" has no PeerServerAddresses — Server Addresses tab would be empty.`);
  }
  console.log(`  PeerServerAddresses: ${active.PeerServerAddresses.join(', ')}`);

  // ── best-effort: confirm the imported/exported services actually
  // replicated over the peering stream. Not a hard failure — replication
  // timing can vary by CONSUL_BIN/image, see README.md's Gotchas.
  try {
    await waitFor(
      async () => {
        const p = (await consul.peering(PRIMARY_SIDE_PEER_NAME)) as {
          StreamStatus?: { ImportedServices?: string[]; ExportedServices?: string[] };
        };
        const imported = p.StreamStatus?.ImportedServices?.includes(IMPORTED_SERVICE);
        const exported = p.StreamStatus?.ExportedServices?.includes(EXPORTED_SERVICE);
        return imported && exported ? true : undefined;
      },
      { timeoutMs: 15_000, label: `"${IMPORTED_SERVICE}"/"${EXPORTED_SERVICE}" to appear in StreamStatus` },
    );
    console.log(`  "${IMPORTED_SERVICE}" imported, "${EXPORTED_SERVICE}" exported — confirmed on "${PRIMARY_SIDE_PEER_NAME}"`);
  } catch (err) {
    console.warn(
      `  WARNING: imported/exported service replication did not confirm within 15s — the peering itself is` +
        ` ACTIVE (and Server Addresses is populated), but service-list replication may lag or (on some` +
        ` CONSUL_BIN builds) not populate at all; see README.md Gotchas. (${(err as Error).message})`,
    );
  }

  // ── 6. Nice-to-have: a second, never-established peer -> stays PENDING,
  // for status-badge variety on the peers list ────────────────────────────
  await consul.generatePeeringToken({ PeerName: PENDING_PEER_NAME });
  const pending = await consul.peering(PENDING_PEER_NAME);
  if (pending.State !== 'PENDING') {
    console.warn(`  WARNING: expected "${PENDING_PEER_NAME}" to be PENDING, got "${pending.State}"`);
  }

  console.log(`  UI: http://localhost:4200/ui/dc1/peers`);
  console.log(`  UI: http://localhost:4200/ui/dc1/peers/${PRIMARY_SIDE_PEER_NAME}`);
}
