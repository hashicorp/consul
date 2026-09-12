/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/roles/navigation.feature

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

module('Acceptance | dc / roles / navigation: Roles Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking a role in the listing and back again',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('role', 3);

      await visit('roles', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/roles'));
      assert.equal(document.title, 'Roles - Consul');

      await page().roles.objectAt(0).role();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.ok(
        currentURL().includes('/dc-1/acls/roles'),
        `Expected URL to contain /dc-1/acls/roles, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );
});
