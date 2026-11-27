/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceYAML,
  createModels,
  setSettings,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / acls / tokens / own-no-delete: Your current token has no delete buttons',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario('Scenario: On the listing page', async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
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

      await visit('tokens', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));

      const token = page().tokens.objectAt(0);
      await token.actions();
      await token.use();
      await token.confirmUse();
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');

      const expected = nspaceYAML(
        `consul:token: "{\\"AccessorID\\":\\"token\\",\\"SecretID\\":\\"ee52203d-989f-4f7a-ab5a-2bef004164ca\\",\\"Namespace\\":\\"@namespace\\",\\"Partition\\":\\"default\\"}"`,
        nspace
      );
      assert.strictEqual(window.localStorage.getItem('consul:token'), expected['consul:token']);

      await token.actions();
      assert
        .dom('.consul-token-list [data-test-tabular-row] [data-test-delete]')
        .doesNotExist('the current token has no delete action');

      await visit('token', { dc: 'dc-1', token: 'token' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens/token'));
      assert.dom('[data-test-confirm-delete]').doesNotExist('there is no delete confirmation');
    });
  }
);
