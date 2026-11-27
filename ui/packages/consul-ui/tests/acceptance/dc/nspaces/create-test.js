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
  submit,
  currentURL,
  fillInForm,
  fillInFields,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const visitCreate = function (nspace) {
  createModels('dc', 1, 'datacenter');
  return visit('nspace', { dc: 'datacenter' }, { nspace });
};

module('Acceptance | dc / nspaces / create', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Visiting the page without error and the title is correct',
    async function (assert, nspace) {
      await visitCreate(nspace);

      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/namespaces/create'));
      assert.equal(document.title, 'New Namespace - Consul');
    },
    { onlyNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Creating a role uses an inline form',
    async function (assert, nspace) {
      await visitCreate(nspace);

      await page().roles.create();
      assert.dom('[data-test-role-creator]').exists();
      await fillInForm('role', { Name: 'my-role', Description: 'My role description' });
      await page().roles.form.cancel();
      assert.dom('[data-test-role-creator]').doesNotExist();
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/namespaces/create'));
    },
    { onlyNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Test we can create a Namespace',
    async function (assert, nspace) {
      await visitCreate(nspace);

      await fillInFields({ Name: 'namespace-name', Description: 'namespace description' });
      await submit();

      requestMade(assert, 'PUT', '/v1/namespace/namespace-name?dc=datacenter', {
        body: { Name: 'namespace-name', Description: 'namespace description' },
      });
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/namespaces'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { onlyNamespaceable: true }
  );
});
