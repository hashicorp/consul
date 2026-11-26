/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/tokens/clone.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  visit,
  page,
  currentURL,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / tokens / clone: Cloning an ACL token', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'datacenter');
    createModels('token', 1, {
      AccessorID: 'token',
      SecretID: 'ee52203d-989f-4f7a-ab5a-2bef004164ca',
      Legacy: null,
    });
  });

  nspaceScenario(
    'Scenario: Cloning an ACL token from the listing page',
    async function (assert, nspace) {
      await visit('tokens', { dc: 'datacenter' }, { nspace });

      await page().tokens.objectAt(0).actions();
      await page().tokens.objectAt(0).clone();

      requestMade(
        assert,
        'PUT',
        nspaceEndpoint('/v1/acl/token/token/clone?dc=datacenter&ns=@!namespace', nspace)
      );
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    }
  );

  nspaceScenario(
    'Scenario: Using an ACL token from the detail page',
    async function (assert, nspace) {
      await visit('token', { dc: 'datacenter', token: 'token' }, { nspace });

      await page().clone();

      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    }
  );
});
