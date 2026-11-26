/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/kvs/edit.feature.
//
// The `.feature` had two scenarios titled "Viewing a Session attached to a KV";
// the second (no session) is titled "Viewing a KV without a Session".

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
  requestNotMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / kvs / edit: KV Viewing', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'datacenter');
  });

  nspaceScenario(
    'Scenario: Viewing a KV with a URL unsafe character',
    async function (assert, nspace) {
      createModels('kv', 1, { Key: '@key' });

      await visit('kv', { dc: 'datacenter', kv: '@key' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/%40key/edit'));
      assert.strictEqual(page().kv.Key, '@key');
    }
  );

  nspaceScenario('Scenario: Viewing a Session attached to a KV', async function (assert, nspace) {
    createModels('kv', 1, { Key: 'key', Session: 'session-id' });

    await visit('kv', { dc: 'datacenter', kv: 'key' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/key/edit'));
    assert.strictEqual(page().session.ID, 'session-id');
  });

  nspaceScenario('Scenario: Viewing a KV without a Session', async function (assert, nspace) {
    createModels('kv', 1, { Key: 'another-key', Session: null });

    await visit('kv', { dc: 'datacenter', kv: 'another-key' }, { nspace });
    assert.dom('[data-test-session]').doesNotExist('there is no session');
  });

  nspaceScenario('Scenario: Viewing a kv with no write access', async function (assert, nspace) {
    createModels('kv', 1, { Key: 'key', Session: 'session-id' });
    setPermissions({ key: { write: false }, session: { read: false } });

    await visit('kv', { dc: 'datacenter', kv: 'key' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/key/edit'));
    assert.dom('[data-test-create]').doesNotExist('there is no create button');
    assert.dom('[data-test-session]').doesNotExist('the session is not shown');
    assert.ok(page().session.warning, 'shows the session warning');
  });

  nspaceScenario('Scenario: Viewing a kv with no read access', async function (assert, nspace) {
    createModels('kv', 1, { Key: 'key' });
    setPermissions({ key: { write: false, read: false } });

    await visit('kv', { dc: 'datacenter', kv: 'key' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/key/edit'));
    assert.strictEqual(page().error.status, '403');
    requestNotMade(assert, 'GET', '/v1/kv/key?dc=datacenter');
  });

  // Make sure we can view KVs that have similar names to sections in the UI
  ['services', 'nodes', 'intentions', 'kvs'].forEach((name) => {
    nspaceScenario(`Scenario: I have KV called ${name}`, async function (assert, nspace) {
      createModels('kv', 1, { Key: name });

      await visit('kv', { dc: 'datacenter', kv: name }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/kv/${name}/edit`));
    });
  });
});
