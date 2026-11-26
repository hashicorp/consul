/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/error.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  visit,
  page,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / error', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 2, ['dc-1', 'dc-2']);
    respondWith('/v1/internal/ui/services', { status: 500 });
  });

  nspaceScenario(
    "Scenario: Arriving at the service page that doesn't exist",
    async function (assert, nspace) {
      await visit('services', { dc: 'non-existent-datacenter' }, { nspace });
      assert.strictEqual(page().error.status, '500');
    }
  );

  nspaceScenario(
    'Scenario: Arriving at the service page',
    async function (assert, nspace) {
      await visit('services', { dc: 'dc-1' }, { nspace });
      assert.strictEqual(page().error.status, '500');
      // This is the actual step that works slightly differently when running
      // through namespaces as the dc menu says 'Error', which is still kind of ok
      await page().navigation.dc();
      assertVisibleCount(assert, page().navigation.dcs, 2, 'datacenter models');
    },
    { notNamespaceable: true }
  );
});
