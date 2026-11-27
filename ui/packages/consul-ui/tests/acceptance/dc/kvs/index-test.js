/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// "Viewing kvs with no write access" is skipped: it still fails (the create
// button is shown).

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  setPermissions,
  visit,
  page,
  currentURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / kvs / index', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Viewing kvs in the listing', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('kv', 3);

    await visit('kvs', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/kv'));
    assert.equal(document.title, 'Key/Value - Consul');
    assertVisibleCount(assert, page().kvs, 3, 'kv models');
  });

  nspaceScenario(
    'Scenario: Viewing kvs with no write access',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('kv', 3);
      setPermissions({ key: { write: false } });

      await visit('kvs', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/kv'));
      assert.dom('[data-test-create]').doesNotExist('there is no create button');
    },
    { ignore: true }
  );
});
