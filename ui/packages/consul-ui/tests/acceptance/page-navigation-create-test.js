/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of the "Clicking create in the [Model] listing" scenarios
// from tests/acceptance/page-navigation.feature (intentions / tokens / policies).

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  pages,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | page-navigation / create-flows', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking create in the intentions listing',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc1');

      await visit('intentions', { dc: 'dc1' }, { nspace });

      await pages.intentions.create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/intentions/create'));

      await click('[data-test-breadcrumb-item]:first-child a');
      assert.ok(
        currentURL().includes('/dc1/intentions'),
        `Expected URL to contain /dc1/intentions, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Clicking create in the tokens listing',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc1');

      await visit('tokens', { dc: 'dc1' }, { nspace });

      await pages.tokens.create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/acls/tokens/create'));

      await click('[data-test-breadcrumb-item]:first-child a');
      assert.ok(
        currentURL().includes('/dc1/acls/tokens'),
        `Expected URL to contain /dc1/acls/tokens, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Clicking create in the policies listing',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc1');

      await visit('policies', { dc: 'dc1' }, { nspace });

      await pages.policies.create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/acls/policies/create'));

      await click('[data-test-breadcrumb-item]:first-child a');
      assert.ok(
        currentURL().includes('/dc1/acls/policies'),
        `Expected URL to contain /dc1/acls/policies, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );
});
