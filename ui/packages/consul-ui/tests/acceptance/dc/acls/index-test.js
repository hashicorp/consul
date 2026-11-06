/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/index.feature

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / index: acl forwarding', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Arriving at the acl index page with no other url info',
    async function (assert, nspace) {
      createModels('dc', 1, 'datacenter');

      await visit('acls', { dc: 'datacenter' }, { nspace });

      assert.strictEqual(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
    },
    { notNamespaceable: true }
  );
});
