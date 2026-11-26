/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nspaces/manage.feature.
// The feature was `@onlyNamespaceable` (Enterprise only), as before; its single
// scenario was untitled (reported as "Scenario: ") and is now titled.

import { module } from 'qunit';

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

// A page-object property that may not exist (yadda's "I don't see ...").
const isShown = function (get) {
  try {
    return !!get();
  } catch (e) {
    return false;
  }
};

module('Acceptance | dc / nspaces / manage : Managing Namespaces', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Moving between services and namespace management',
    async function (assert, nspace) {
      setSettings({
        'consul:token': { SecretID: 'secret', AccessorID: 'accessor', Namespace: 'default' },
      });
      createModels('dc', 1, ['dc-1']);
      createModels(
        'service',
        6,
        [0, 1, 2, 3, 4, 5].map((i) => ({ Name: `Service-${i}`, Kind: null }))
      );

      await visit('services', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
      assertVisibleCount(assert, page().services, 6, 'service models');

      // In order to test this properly you have to click around a few times
      // between services and nspace management
      const { navigation } = page();
      await navigation.nspace();
      await navigation.manageNspaces();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/namespaces'));
      assert.notOk(
        isShown(() => navigation.manageNspacesIsVisible),
        'the menu is closed'
      );

      await navigation.services();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
      await navigation.manageNspaces();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/namespaces'));
      assert.notOk(
        isShown(() => navigation.manageNspacesIsVisible),
        'the menu is closed'
      );
    },
    { onlyNamespaceable: true }
  );
});
