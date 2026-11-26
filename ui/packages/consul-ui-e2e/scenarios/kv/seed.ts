/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * The shallow, hand-written part of the tree — everyday shapes at 1-4 levels:
 *
 *   readme                                    (standalone top-level key)
 *   maintenance-mode                          (standalone top-level key)
 *   build-number                               (standalone top-level key)
 *   config/timeout                            (1-level folder)
 *   config/retries                            (1-level folder)
 *   app/settings/theme                        (2-level folder)
 *   app/settings/locale                       (2-level folder)
 *   app/services/billing/owner                (3-level folder)
 *   app/services/billing/flags/dark-launch    (4-level folder)
 *   archive/2023/                             (explicitly empty folder)
 *
 * consul-ui derives folders from `/`-separated key prefixes on the fly, so
 * populated folders (`config/`, `app/settings/`, ...) never need their own
 * marker key — only `archive/2023/` does, since otherwise nothing would
 * ever exist at that prefix for the UI to list.
 */
const BASE_ENTRIES: Record<string, string> = {
  readme: 'Sample KV data seeded by the kv scenario for the Key/Value browser UI.',
  'maintenance-mode': 'false',
  'build-number': '42',

  'config/timeout': '30s',
  'config/retries': '3',

  'app/settings/theme': 'dark',
  'app/settings/locale': 'en-US',

  'app/services/billing/owner': 'team-payments',
  'app/services/billing/flags/dark-launch': 'true',

  // Explicitly empty folder: written with an empty value, no other keys
  // exist under this prefix.
  'archive/2023/': '',
};

/**
 * A single chain 15 folders deep (`deep/` is level 1, `level-02/`..`level-15/`
 * the rest), for the far end of the tree's indentation — the list indents by
 * 20px per level, so the deepest row sits ~280px in.
 *
 * Only some levels carry a key next to their child folder, so the chain has
 * both *mixed* levels (a key and a folder side by side, which is where the
 * sort-by-Kind grouping actually shows) and *folder-only* levels.
 */
const DEEP_DEPTH = 15;
const DEEP_LEVELS_WITH_KEYS = new Set([1, 3, 7, 12]);

const DEEP_SEGMENTS = [
  'deep',
  ...Array.from({ length: DEEP_DEPTH - 1 }, (_, i) => `level-${String(i + 2).padStart(2, '0')}`),
];

/**
 * Names long enough to overflow the tree's "Key name" column. Consul caps a
 * key path at 512 chars, which the longest of these (two long folders plus a
 * long key) stays well inside.
 */
const LONG_FOLDER_NAME =
  'a-folder-with-a-deliberately-long-name-that-overflows-the-key-value-browser-tree-column';
const LONG_NESTED_FOLDER_NAME =
  'another-very-long-folder-name-nested-inside-the-first-one-to-double-up-on-the-overflow';
const LONG_KEY_NAME =
  'a-key-with-a-deliberately-long-name-that-should-truncate-rather-than-wrap-its-table-row';

function deepEntries(): Record<string, string> {
  const out: Record<string, string> = {};
  let prefix = '';

  DEEP_SEGMENTS.forEach((segment, index) => {
    prefix += `${segment}/`;
    const level = index + 1;
    if (DEEP_LEVELS_WITH_KEYS.has(level)) {
      out[`${prefix}depth`] = String(level);
    }
  });

  // `prefix` is now the deepest folder, deep/level-02/.../level-15/
  out[`${prefix}deepest-key`] = 'Fifteen folders deep — the most indented row in the tree.';
  // Worst case for the column: maximum indentation *and* a long name.
  out[`${prefix}${LONG_KEY_NAME}`] = 'Deepest indentation and an overflowing name at once.';

  return out;
}

function longNameEntries(): Record<string, string> {
  return {
    [`${LONG_FOLDER_NAME}/short-key`]: 'A short key inside a folder with a very long name.',
    [`${LONG_FOLDER_NAME}/${LONG_KEY_NAME}`]: 'Both the folder and the key have overflowing names.',
    [`${LONG_FOLDER_NAME}/${LONG_NESTED_FOLDER_NAME}/nested-key`]: 'Two long-named folders deep.',
  };
}

const ENTRIES: Record<string, string> = {
  ...BASE_ENTRIES,
  ...deepEntries(),
  ...longNameEntries(),
};

export async function seed(consul: ConsulClient): Promise<void> {
  for (const [key, value] of Object.entries(ENTRIES)) {
    await consul.putKV(key, value);
  }

  const stored = await consul.kv();
  const expectedKeys = Object.keys(ENTRIES);
  const missing = expectedKeys.filter((key) => !stored.some((entry) => entry.Key === key));
  if (missing.length > 0) {
    throw new Error(`expected KV keys not found after seeding: ${missing.join(', ')}`);
  }

  // Consul returns Value: null (not "") for a key written with an empty body.
  const emptyFolder = stored.find((entry) => entry.Key === 'archive/2023/');
  if (!emptyFolder || emptyFolder.Value !== null) {
    throw new Error(`expected archive/2023/ to exist as an empty folder marker, got ${JSON.stringify(emptyFolder)}`);
  }

  // The deep chain is the one thing here Consul could silently truncate or
  // reject on length, so confirm the deepest key really sits 15 folders down.
  const deepest = `${DEEP_SEGMENTS.join('/')}/deepest-key`;
  const deepestEntry = stored.find((entry) => entry.Key === deepest);
  if (!deepestEntry) {
    throw new Error(`expected the deepest key to exist at ${deepest}`);
  }
  const depth = deepest.split('/').length - 1;
  if (depth !== DEEP_DEPTH) {
    throw new Error(`expected the deepest key nested ${DEEP_DEPTH} folders deep, got ${depth}`);
  }
}
