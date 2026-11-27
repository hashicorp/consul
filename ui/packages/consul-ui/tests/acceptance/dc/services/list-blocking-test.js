/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// After the edit to 5 services this still waits for 3, as the original test
// did, so that step only checks the list still renders.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  setLatency,
  visit,
  page,
  currentURL,
  waitForVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / list blocking', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Viewing the listing pages for service',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels(
        'service',
        3,
        [0, 1, 2].map((i) => ({ Name: `Service-${i}`, Kind: null }))
      );
      setLatency(100);

      await visit('services', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
      await waitForVisibleCount(assert, page().services, 3, 'service models');

      createModels('service', 5);
      await waitForVisibleCount(assert, page().services, 3, 'service models');
      createModels('service', 1);
      await waitForVisibleCount(assert, page().services, 1, 'service models');
      createModels('service', 0);
      await waitForVisibleCount(assert, page().services, 0, 'service models');
    }
  );
});
