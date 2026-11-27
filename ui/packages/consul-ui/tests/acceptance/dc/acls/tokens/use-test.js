/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceYAML,
  createModels,
  setSettings,
  visit,
  page,
} from 'consul-ui/tests/helpers/acceptance';

// The settings the app should store once the "token" token is in use.
const expectedToken = function (nspace) {
  return nspaceYAML(
    `consul:token: "{\\"AccessorID\\":\\"token\\",\\"SecretID\\":\\"ee52203d-989f-4f7a-ab5a-2bef004164ca\\",\\"Namespace\\":\\"@namespace\\",\\"Partition\\":\\"default\\"}"`,
    nspace
  )['consul:token'];
};

module('Acceptance | dc / acls / tokens / use: Using an ACL token', function (hooks) {
  setupAcceptanceTest(hooks);

  // Background
  const setup = function (nspace) {
    createModels('dc', 1, 'datacenter');
    createModels(
      'token',
      1,
      nspaceYAML(
        `
AccessorID: token
SecretID: ee52203d-989f-4f7a-ab5a-2bef004164ca
Namespace: @!namespace
`,
        nspace
      )
    );
    setSettings({
      'consul:token': {
        SecretID: 'secret',
        AccessorID: 'accessor',
        Namespace: 'default',
        Partition: 'default',
      },
    });
  };

  nspaceScenario(
    'Scenario: Using an ACL token from the listing page',
    async function (assert, nspace) {
      setup(nspace);
      await visit('tokens', { dc: 'datacenter' }, { nspace });

      const token = page().tokens.objectAt(0);
      await token.actions();
      await token.use();
      await token.confirmUse();
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');

      assert.strictEqual(window.localStorage.getItem('consul:token'), expectedToken(nspace));
    }
  );

  nspaceScenario(
    'Scenario: Using an ACL token from the detail page',
    async function (assert, nspace) {
      setup(nspace);
      await visit('token', { dc: 'datacenter', token: 'token' }, { nspace });

      await page().use();
      await page().confirmUse();
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');

      assert.strictEqual(window.localStorage.getItem('consul:token'), expectedToken(nspace));
    }
  );
});
