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
  createModels,
  api,
  visit,
  page,
  currentURL,
  nspaceURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / roles / navigation: Roles Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking a role in the listing and back again',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      api.server.createList('role', 3);

      await visit('roles', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/roles'));
      assert.equal(document.title, 'Roles - Consul');
      assertVisibleCount(assert, page().roles, 3, 'roles');

      await page().roles.objectAt(0).role();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/roles'));
    }
  );
});
