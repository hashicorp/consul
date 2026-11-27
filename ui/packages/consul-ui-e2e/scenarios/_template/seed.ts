/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Register the data this use-case needs. Called after the agent is ready
 * (and, if `requiresConnect` is set in scenario.json, after the Connect CA).
 *
 * Keep it declarative — reuse the ConsulClient helpers rather than raw fetch.
 */
export async function seed(consul: ConsulClient): Promise<void> {
  await consul.registerService({ Name: 'example', Port: 8080 });
}
