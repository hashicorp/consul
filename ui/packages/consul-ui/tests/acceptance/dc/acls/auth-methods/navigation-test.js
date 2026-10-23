/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/auth-methods/navigation.feature

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

module(
  'Acceptance | dc / acls / auth-methods / navigation: Auth Methods Navigation',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: Clicking an auth-method in the listing and back again',
      async function (assert, nspace) {
        api.server.createList('dc', 1, 'dc-1');
        api.server.createList('authMethod', 3);

        await visit('authMethods', { dc: 'dc-1' }, { nspace });

        assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/auth-methods'));
        assert.equal(document.title, 'Auth Methods - Consul');

        // Navigate into the first auth-method
        await page().authMethods.objectAt(0).authMethod();

        // Click the first breadcrumb (list page) to go back
        await click('[data-test-breadcrumb-item]:first-child a');

        assert.ok(
          currentURL().includes('/dc-1/acls/auth-methods'),
          `Expected URL to contain /dc-1/acls/auth-methods, got ${currentURL()}`
        );
      },
      { notNamespaceable: true }
    );
  }
);
