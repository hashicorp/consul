/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Ported from scripts/dev-lock-sessions.sh.
 *
 * Creates many sessions on the live agent node (the session API requires the
 * node to be a live agent — externally-registered catalog nodes cannot hold
 * sessions) with varied Name / Behavior / TTL / LockDelay values so every
 * column of the Lock Sessions table has meaningful data.
 */
const NAMES = [
  'web-lock', 'db-lock', 'cache-lock', 'api-gateway', 'scheduler',
  'background-job', 'kv-lock', 'leader-lock', 'health-check-lock', 'service-lock',
];
const BEHAVIORS = ['release', 'delete'];
const TTLS = ['30s', '60s', '120s', '45s', '90s', '', '10s', '300s', '60s', ''];
// nanoseconds
const LOCK_DELAYS = [
  15_000_000_000, 120_000, 18_000_000, 5_000_000_000, 30_000_000_000,
  15_000_000_000, 1_000_000_000, 120_000, 18_000_000, 15_000_000_000,
];

const SESSION_COUNT = Number(process.env.SESSION_COUNT ?? 32);

export async function seed(consul: ConsulClient): Promise<void> {
  const node = await consul.nodeName();

  for (let i = 1; i <= SESSION_COUNT; i++) {
    const name = `${NAMES[(i - 1) % NAMES.length]}-${i}`;
    const behavior = BEHAVIORS[i % 2];
    const ttl = TTLS[(i - 1) % TTLS.length];
    const lockDelay = LOCK_DELAYS[(i - 1) % LOCK_DELAYS.length];

    const session: Record<string, unknown> = { Node: node, Name: name, Behavior: behavior, LockDelay: lockDelay };
    if (ttl) session.TTL = ttl; // omit entirely when empty so Consul uses no TTL

    await consul.createSession(session);
  }

  const total = (await consul.nodeSessions(node)).length;
  if (total < SESSION_COUNT) {
    throw new Error(`expected >= ${SESSION_COUNT} sessions on ${node}, found ${total}`);
  }
}
