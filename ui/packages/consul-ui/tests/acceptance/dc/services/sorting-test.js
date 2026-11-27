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
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Sorting the service list by name', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels(
      'service',
      4,
      ['B', 'D', 'A', 'C'].map((x) => ({ Name: `Service-${x}`, Kind: null }))
    );

    await visit('services', { dc: 'dc-1' }, { nspace });
    // ascending (A-Z)
    await page().sort.name();
    assertPropertiesVertically(
      assert,
      page().services,
      'name',
      ['Service-A', 'Service-B', 'Service-C', 'Service-D'],
      'services'
    );
    // descending (Z-A)
    await page().sort.name();
    assertPropertiesVertically(
      assert,
      page().services,
      'name',
      ['Service-D', 'Service-C', 'Service-B', 'Service-A'],
      'services'
    );
  });
});
