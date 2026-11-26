/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Enterprise-only: seeds the Admin Partitions page
 * (`/ui/dc1/partitions`, `app/templates/dc/partitions/index.hbs` in
 * consul-ui) with named partitions, deliberately varied on the one column
 * the list actually renders beyond Name: Description
 * (`consul-ui/app/components/consul/partition/list`, whose `<:details>`
 * block is conditional on `item.Description`).
 *
 * Every agent already has an implicit "default" partition (not created here,
 * same as the built-in "default" namespace) — it can't be deleted
 * (`abilities/partition.js#canDelete`), so it's left alone as the
 * un-deletable baseline. The three below sort alphabetically around it
 * (Name:asc is the page's default sort):
 *
 *   billing   — has a Description
 *   default   — (implicit, un-deletable)
 *   finance   — has a Description
 *   security  — no Description (exercises the empty-details branch, and is
 *               left with no children so it's a safe target for a
 *               create-then-delete test)
 *
 * On top of those, `team-01`..`team-50` give the list real volume — for
 * scroll behaviour and for search/sort assertions at scale (e.g. searching
 * "team" or "Bulk-seeded" should match all 50, `Name:asc` should list them
 * in `team-01`..`team-50` order — zero-padded so string sort matches numeric
 * order). They sort after `security` (alphabetically 't' > 's'), so the full
 * list order is: billing, default, finance, security, team-01, ..., team-50.
 */
const BULK_COUNT = 50;

export async function seed(consul: ConsulClient): Promise<void> {
  await consul.createPartition({ Name: 'billing', Description: 'Billing and payments team workloads' });
  await consul.createPartition({ Name: 'finance', Description: 'Finance and ledger team workloads' });
  await consul.createPartition({ Name: 'security' });

  const bulkNames: string[] = [];
  for (let i = 1; i <= BULK_COUNT; i++) {
    const name = `team-${String(i).padStart(2, '0')}`;
    bulkNames.push(name);
    await consul.createPartition({
      Name: name,
      Description: `Bulk-seeded partition ${i} of ${BULK_COUNT} for list volume/search testing.`,
    });
  }

  // Cheap assertion that the data landed before Playwright starts.
  const partitions = (await consul.partitions()).map((p) => p.Name);
  for (const name of ['billing', 'finance', 'security', ...bulkNames]) {
    if (!partitions.includes(name)) {
      throw new Error(`"${name}" partition missing — found: ${partitions.join(', ')}`);
    }
  }
}
