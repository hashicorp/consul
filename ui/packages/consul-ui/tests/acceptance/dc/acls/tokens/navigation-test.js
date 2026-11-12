/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/tokens/navigation.feature

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

module('Acceptance | dc / tokens / navigation: Tokens Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking a token in the listing and back again',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      api.server.createList('token', 3);

      await visit('tokens', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
      assert.equal(document.title, 'Tokens - Consul');
      assertVisibleCount(assert, page().tokens, 3, 'tokens');

      await page().tokens.objectAt(0).token();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
    }
  );
});
