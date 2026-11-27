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
  setSettings,
  visit,
  page,
  currentURL,
  assertVisibleCount,
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / nspaces / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Sorting Namespaces',
    async function (assert, nspace) {
      setSettings({
        'consul:token': { SecretID: 'secret', AccessorID: 'accessor', Namespace: 'default' },
      });
      createModels('dc', 1, 'dc-1');
      createModels(
        'nspace',
        6,
        [5, 3, 1, 4, 2, 6].map((i) => ({ Name: `nspace-${i}` }))
      );

      await visit('nspaces', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/namespaces'));
      assertVisibleCount(assert, page().nspaces, 6, 'nspace models');

      await page().sort.selected();
      await page().sort.options.objectAt(1).button();
      assertPropertiesVertically(
        assert,
        page().nspaces,
        'name',
        [6, 5, 4, 3, 2, 1].map((i) => `nspace-${i}`),
        'nspaces'
      );

      await page().sort.selected();
      await page().sort.options.objectAt(0).button();
      assertPropertiesVertically(
        assert,
        page().nspaces,
        'name',
        [1, 2, 3, 4, 5, 6].map((i) => `nspace-${i}`),
        'nspaces'
      );
    },
    { onlyNamespaceable: true }
  );
});
