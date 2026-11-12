/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/index-forwarding.feature

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | index-forwarding', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Arriving at the index page when there is only one datacenter',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc1');

      await visit('index', {}, { nspace });

      assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc1/services'));
    },
    { notNamespaceable: true }
  );
});
