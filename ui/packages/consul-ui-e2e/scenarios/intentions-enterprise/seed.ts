/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Enterprise-only companion to `scenarios/intentions`: registers a second
 * namespace and a second admin partition, then intentions that cross them —
 * the only way to exercise the Source/Destination Namespace and Partition
 * pickers in the create/edit form (they only render when
 * `can 'choose nspaces'` / `can 'choose partitions'` are true, which require
 * a licensed Enterprise agent).
 *
 * Namespaces/partitions:
 *   default/default   — web, api      (parity with the CE scenario)
 *   default/billing    — payments      (cross-namespace target)
 *   finance/default    — ledger        (cross-partition target)
 *
 * Intentions:
 *   web (default/default) -> api      (default/default)  Allow
 *   web (default/default) -> payments (default/billing)   Deny   — cross-namespace
 *   web (default/default) -> ledger   (finance/default)   Allow  — cross-partition
 *
 * Cross-partition intentions are only permitted from the primary datacenter
 * (which a single-agent dev cluster always is) — the UI's own "Cross-partition
 * communication not supported outside of the primary datacenter" alert
 * documents this. If the API ever rejects it here, that alert is the reason.
 */
export async function seed(consul: ConsulClient): Promise<void> {
  await consul.createNamespace({ Name: 'billing' });
  await consul.createPartition({ Name: 'finance' });

  await consul.registerService({ Name: 'web', Port: 9090 });
  await consul.registerService({ Name: 'api', Port: 9091 });
  await consul.registerService({ Name: 'payments', Port: 9092, Namespace: 'billing' } as Record<
    string,
    unknown
  >);
  // `ledger` lives in the "finance" partition. A single agent belongs to
  // exactly one partition — `/v1/agent/service/register` (registerService)
  // 400s with "request targets partition does not match agent partition" for
  // any other partition; a real "finance" service would need its own client
  // agent started with `-partition=finance`. The server-side catalog API
  // (registerCatalogNode) has no such restriction — it registers a virtual
  // node directly into the target partition, agentless.
  await consul.registerCatalogNode({
    Node: 'node-primary-finance',
    Address: '127.0.0.1',
    Partition: 'finance',
    Service: { Service: 'ledger', Port: 9093, Partition: 'finance' },
  });

  await consul.registerIntention({ SourceName: 'web', DestinationName: 'api', Action: 'allow' });

  await consul.registerIntention({
    SourceName: 'web',
    DestinationName: 'payments',
    DestinationNS: 'billing',
    Action: 'deny',
  });

  await consul.registerIntention({
    SourceName: 'web',
    DestinationName: 'ledger',
    DestinationPartition: 'finance',
    Action: 'allow',
  });

  // Cheap assertions that the data landed before Playwright starts.
  const namespaces = (await consul.namespaces()).map((n) => n.Name);
  if (!namespaces.includes('billing')) {
    throw new Error(`"billing" namespace missing — found: ${namespaces.join(', ')}`);
  }
  const partitions = (await consul.partitions()).map((p) => p.Name);
  if (!partitions.includes('finance')) {
    throw new Error(`"finance" partition missing — found: ${partitions.join(', ')}`);
  }
  const seeded = await consul.intentions();
  if (seeded.length < 3) {
    throw new Error(`expected 3 seeded intentions, found ${seeded.length}`);
  }
}
