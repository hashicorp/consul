/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/dc-switch.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / dc-switch : Switching Datacenters', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Seeing all services when switching datacenters',
    async function (assert, nspace) {
      createModels('dc', 2, ['dc-1', 'dc-2']);
      createModels(
        'service',
        6,
        [0, 1, 2, 3, 4, 5].map((i) => ({ Name: `Service-${i}`, Kind: null }))
      );

      await visit('services', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
      assertVisibleCount(assert, page().services, 6, 'service models');

      const { navigation } = page();
      await navigation.dc();
      for (const [i, dc] of [
        [1, 'dc-2'],
        [0, 'dc-1'],
        [1, 'dc-2'],
      ]) {
        await navigation.dcs.objectAt(i).name();
        assert.equal(currentURL(), nspaceURL(nspace, `/${dc}/services`));
        assertVisibleCount(assert, page().services, 6, 'service models');
      }
    }
  );
});
