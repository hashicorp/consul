/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/navigation.feature

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / navigation: Services Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking a service in the listing and back again',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);

      await visit('services', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
      assert.equal(document.title, 'Services - Consul');

      await page().services.objectAt(0).service();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.ok(
        currentURL().includes('/dc-1/services'),
        `Expected URL to contain /dc-1/services, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Clicking a peered service in the listing and back again',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);

      await visit('services', { dc: 'dc-1' }, { nspace });

      await page().services.objectAt(0).service();

      assert.ok(
        currentURL().match(/\/dc-1\/services\/.+/),
        `Expected service detail URL, got ${currentURL()}`
      );

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.ok(
        currentURL().includes('/dc-1/services'),
        `Expected URL to contain /dc-1/services, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );
});
