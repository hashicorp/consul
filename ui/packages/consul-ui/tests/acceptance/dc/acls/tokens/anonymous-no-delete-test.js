/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

const ANONYMOUS = '00000000-0000-0000-0000-000000000002';

module(
  'Acceptance | dc / acls / tokens / anonymous-no-delete: The anonymous token has no delete buttons',
  function (hooks) {
    setupAcceptanceTest(hooks);

    hooks.beforeEach(function () {
      createModels('dc', 1, 'dc-1');
      createModels('token', 1, { AccessorID: ANONYMOUS, Policies: null });
    });

    nspaceScenario('Scenario: On the listing page', async function (assert, nspace) {
      await visit('tokens', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));

      await page().tokens.objectAt(0).actions();
      assert
        .dom('.consul-token-list [data-test-tabular-row] [data-test-delete]')
        .doesNotExist('the anonymous token has no delete action');
    });

    nspaceScenario('Scenario: On the detail page', async function (assert, nspace) {
      await visit('token', { dc: 'dc-1', token: ANONYMOUS }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, `/dc-1/acls/tokens/${ANONYMOUS}`));

      assert.dom('[data-test-confirm-delete]').doesNotExist('there is no delete confirmation');
    });
  }
);
