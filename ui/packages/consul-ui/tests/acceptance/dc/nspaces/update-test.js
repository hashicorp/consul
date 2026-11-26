/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nspaces/update.feature.
// The feature was `@onlyNamespaceable` (Enterprise only), as before.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  respondWith,
  visit,
  submit,
  currentURL,
  fillInFields,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const URL = '/v1/namespace/namespace?dc=datacenter';

// Background: the edit form for the "namespace" namespace.
const visitEdit = async function (assert, nspace) {
  createModels('dc', 1, 'datacenter');
  createModels('nspace', 1, { Name: 'namespace', Description: 'empty', PolicyDefaults: null });

  await visit('nspace', { dc: 'datacenter', namespace: 'namespace' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/namespaces/namespace'));
  assert.equal(document.title, 'Edit namespace - Consul');
};

module('Acceptance | dc / nspaces / update: Nspace Update', function (hooks) {
  setupAcceptanceTest(hooks);

  ['description', 'description with spaces'].forEach((Description) => {
    nspaceScenario(
      `Scenario: Update to ${Description}`,
      async function (assert, nspace) {
        await visitEdit(assert, nspace);

        await fillInFields({ Description });
        await submit();

        requestMade(assert, 'PUT', URL, { body: { Description } });
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/namespaces'));
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      },
      { onlyNamespaceable: true }
    );
  });

  nspaceScenario(
    'Scenario: There was an error saving the key',
    async function (assert, nspace) {
      await visitEdit(assert, nspace);

      respondWith(URL, { status: 500 });
      await submit();

      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/namespaces/namespace'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
    },
    { onlyNamespaceable: true }
  );
});
