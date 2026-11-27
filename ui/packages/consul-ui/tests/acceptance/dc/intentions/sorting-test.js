/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  page,
  assertVisibleCount,
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / intentions / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Sorting Intentions',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('intention', 3, [
        { SourceName: 'service-B' },
        { SourceName: 'service-D' },
        { SourceName: 'service-A' },
      ]);

      await visit('intentions', { dc: 'dc-1' }, { nspace });
      const intentions = page().intentionList.intentions;
      assertVisibleCount(assert, intentions, 3, 'intention models');

      // ascending (A-Z)
      await page().sort.source();
      assertPropertiesVertically(
        assert,
        intentions,
        'source',
        ['service-A', 'service-B', 'service-D'],
        'intentions'
      );

      // descending (Z-A)
      await page().sort.source();
      assertPropertiesVertically(
        assert,
        intentions,
        'source',
        ['service-D', 'service-B', 'service-A'],
        'intentions'
      );
    },
    { notNamespaceable: true }
  );
});
