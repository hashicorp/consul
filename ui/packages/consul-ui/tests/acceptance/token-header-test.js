/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/token-header.feature (feature-level @notNamespaceable)

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  fillInForm,
  find,
  visit,
  page,
  currentURL,
  nspaceURL,
  nspaceEndpoint,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | token-header', function (hooks) {
  setupAcceptanceTest(hooks);

  const servicesRequest = (nspace) =>
    nspaceEndpoint('/v1/internal/ui/services?dc=dc1&ns=@namespace', nspace);

  nspaceScenario(
    'Scenario: Arriving at the service page having not set a token previously',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc1');

      await visit('services', { dc: 'dc1' }, { nspace });

      assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc1/services'));
      requestMade(assert, 'GET', servicesRequest(nspace), { headers: { 'X-Consul-Token': '' } });
    },
    { notNamespaceable: true }
  );

  ['token', ''].forEach((token) => {
    nspaceScenario(
      `Scenario: Set the token to ${token || "''"} and then navigate to the service page`,
      async function (assert, nspace) {
        createModels('dc', 1, 'dc1');
        respondWith('/v1/acl/tokens', { status: 403 });

        await visit('tokens', { dc: 'dc1' }, { nspace });
        assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc1/acls/tokens'));

        await page().navigation.authMenu();
        await page().navigation.login();
        await fillInForm('auth', { SecretID: token });
        await find('authdialog.form.submit')();

        await visit('services', { dc: 'dc1' }, { nspace });
        assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc1/services'));
        requestMade(assert, 'GET', servicesRequest(nspace), {
          headers: { 'X-Consul-Token': token },
        });
      },
      { notNamespaceable: true }
    );
  });
});
