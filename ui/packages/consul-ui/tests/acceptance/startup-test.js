/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/startup.feature

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | startup', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: When loading the index.html file into a browser',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');

      assert.strictEqual(currentURL(), '');
      assert.dom(document.documentElement).hasClass('ember-loading');

      await visit('services', { dc: 'dc-1' }, { nspace });

      assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc-1/services'));
      assert.dom(document.documentElement).doesNotHaveClass('ember-loading');
    },
    { notNamespaceable: true }
  );
});
