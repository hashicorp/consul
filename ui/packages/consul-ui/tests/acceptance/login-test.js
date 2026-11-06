/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/login.feature

import { module } from 'qunit';
import { click, fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  enableSSO,
  enablePartitions,
  stubOIDCProvider,
  fillInForm,
  find,
  visit,
  page,
  currentURL,
  nspaceURL,
  nspaceEndpoint,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | login', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Logging into the login page from ACLs tokens',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      respondWith('/v1/acl/tokens', { status: 403 });

      await visit('tokens', { dc: 'dc-1' }, { nspace });
      assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));

      await page().navigation.authMenu();
      await page().navigation.login();
      await fillInForm('auth', { SecretID: 'something' });
      await find('authdialog.form.submit')();

      requestMade(assert, 'GET', '/v1/acl/token/self?dc=dc-1', {
        headers: { 'X-Consul-Token': 'something' },
      });
    }
  );

  nspaceScenario(
    'Scenario: Logging in via SSO',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      enableSSO();
      enablePartitions();
      createModels('oidcProvider', 1, [{ DisplayName: 'Okta', Name: 'okta', Kind: 'okta' }]);

      await visit('services', { dc: 'dc-1' }, { nspace });
      stubOIDCProvider({
        state: 'state-123456789/abcdefghijklmnopqrstuvwxyz',
        code: 'code-abcdefghijklmnopqrstuvwxyz/123456789',
      });

      await page().navigation.authMenu();
      await page().navigation.login();
      await click('[data-test-tab=tab_sso] button');
      assert.dom("[name='partition']").hasValue('default');
      await fillIn('[name=partition]', 'partition');
      await click('.oidc-select button');
      requestMade(
        assert,
        'GET',
        nspaceEndpoint(
          '/v1/internal/ui/oidc-auth-methods?dc=dc-1&ns=@namespace&partition=partition',
          nspace
        )
      );
      await click('.okta-oidc-provider');
      requestMade(
        assert,
        'POST',
        nspaceEndpoint('/v1/acl/oidc/auth-url?dc=dc-1&ns=@!namespace&partition=partition', nspace)
      );
      requestMade(
        assert,
        'POST',
        nspaceEndpoint('/v1/acl/oidc/callback?dc=dc-1&ns=@!namespace&partition=partition', nspace)
      );
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { onlyNamespaceable: true }
  );
});
