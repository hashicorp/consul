/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { fillIn } from '@ember/test-helpers';
import { findOne } from 'ember-cli-page-object/extend';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  setSettings,
  visit,
  page,
  currentURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

// Background: three namespaces, including default.
const visitNspaces = async function (assert, nspace) {
  setSettings({
    'consul:token': { SecretID: 'secret', AccessorID: 'accessor', Namespace: 'default' },
  });
  createModels('dc', 1, 'dc-1');
  createModels('nspace', 3, [
    { Name: 'a-namespace', Description: 'a namespace' },
    { Name: 'default', Description: 'The default namespace' },
    { Name: 'z-namespace', Description: 'z namespace' },
  ]);

  await visit('nspaces', { dc: 'dc-1' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/namespaces'));
  assert.equal(document.title, 'Namespaces - Consul');
};

module('Acceptance | dc / nspaces / index: Nspaces List', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Viewing the namespaces',
    async function (assert, nspace) {
      await visitNspaces(assert, nspace);
      assertVisibleCount(assert, page().nspaces, 3, 'nspace models');
    },
    { onlyNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Searching the nspaces',
    async function (assert, nspace) {
      await visitNspaces(assert, nspace);
      assertVisibleCount(assert, page().nspaces, 3, 'nspace models');

      await fillIn('[name="search"]', 'default');
      assertVisibleCount(assert, page().nspaces, 1, 'nspace models');
      const described = page().nspaces.filter(
        (item) => item.isVisible && item.description === 'The default namespace'
      );
      assert.equal(described.length, 1, 'sees the default namespace');
    },
    { onlyNamespaceable: true }
  );

  nspaceScenario(
    "Scenario: The default namespace can't be deleted",
    async function (assert, nspace) {
      await visitNspaces(assert, nspace);
      assertVisibleCount(assert, page().nspaces, 3, 'nspace models');

      const defaultNspace = page().nspaces.objectAt(1);
      await defaultNspace.actions();
      assert.notOk(
        findOne(defaultNspace).querySelector('[data-test-delete]'),
        'the default namespace has no delete action'
      );
    },
    { onlyNamespaceable: true }
  );
});
