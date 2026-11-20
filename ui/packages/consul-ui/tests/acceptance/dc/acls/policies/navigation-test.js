/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/navigation.feature

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

module('Acceptance | dc / policies / navigation: Policies Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking a policy in the listing and back again',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      api.server.createList('policy', 3);

      await visit('policies', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/policies'));
      assert.equal(document.title, 'Policies - Consul');
      assertVisibleCount(assert, page().policies, 3, 'policies');

      await page().policies.objectAt(0).policy();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/policies'));
    }
  );
});
