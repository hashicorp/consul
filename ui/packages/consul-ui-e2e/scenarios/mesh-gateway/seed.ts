/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Seeds the service-instance "Addresses" tab (HDS migration, see
 * consul-ui/docs/service-instance-addresses-hds-migration.md).
 *
 * That tab is gated to `item.Service.Kind === 'mesh-gateway'` in
 * app/templates/dc/services/instance.hbs, and renders one card per entry of
 * `item.Service.TaggedAddresses`. So we register a mesh-gateway whose
 * TaggedAddresses cover the cases the tab shows:
 *   - a `lan` entry that matches the instance Address:Port -> "(default)" marker
 *   - a `wan` entry (distinct address) -> a plain card
 *   - extra ipv4-suffixed entries for a fuller list
 */
const ADDRESS = '10.0.0.10';
const PORT = 8443;

export async function seed(consul: ConsulClient): Promise<void> {
  await consul.registerService({
    Kind: 'mesh-gateway',
    Name: 'mesh-gateway',
    ID: 'mesh-gateway',
    Address: ADDRESS,
    Port: PORT,
    TaggedAddresses: {
      // Matches instance Address:Port -> the UI marks this one "(default)".
      lan: { Address: ADDRESS, Port: PORT },
      lan_ipv4: { Address: ADDRESS, Port: PORT },
      wan: { Address: '52.10.10.10', Port: 8443 },
      wan_ipv4: { Address: '52.10.10.10', Port: 8443 },
    },
  });

  const svc = await consul.agentService('mesh-gateway');
  if (svc.Kind !== 'mesh-gateway') {
    throw new Error('mesh-gateway not registered with Kind=mesh-gateway — Addresses tab will not appear');
  }
  const tagged = (svc.TaggedAddresses ?? {}) as Record<string, unknown>;
  if (Object.keys(tagged).length === 0) {
    throw new Error('mesh-gateway has no TaggedAddresses — Addresses tab would be empty');
  }
}
