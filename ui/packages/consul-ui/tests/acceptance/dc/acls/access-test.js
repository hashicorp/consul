/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/access.feature

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  disableACLs,
  visit,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / access: ACLs Access', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: ACLs are disabled', async function (assert, nspace) {
    disableACLs();
    createModels('dc', 1, 'dc-1');

    await visit('tokens', { dc: 'dc-1' }, { nspace });

    assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
    assert.dom('[data-test-acls-disabled]').exists();
  });
});
