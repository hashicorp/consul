/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/login-errors.feature

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  visit,
  page,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | login-errors: Login Errors', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: I get any 500 error that is not the specific legacy token cluster one',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      respondWith('/v1/acl/tokens?ns=@namespace', { status: 500 });

      await visit('tokens', { dc: 'dc-1' }, { nspace });

      assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
      assert.strictEqual(page().error.status, '500');
    }
  );
});
