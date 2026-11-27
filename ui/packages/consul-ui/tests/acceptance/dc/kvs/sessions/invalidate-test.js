/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  page,
  currentURL,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const SESSION = 'ee52203d-989f-4f7a-ab5a-2bef004164ca';

// Background: the edit page of a KV holding a lock session.
const visitKV = async function (assert, nspace) {
  createModels('dc', 1, 'datacenter');
  createModels('kv', 1, { Key: 'key' });

  await visit('kv', { dc: 'datacenter', kv: 'key' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/key/edit'));
};

module('Acceptance | dc / kvs / sessions / invalidate: Invalidate Lock Sessions', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Invalidating the lock session', async function (assert, nspace) {
    await visitKV(assert, nspace);

    await page().session.delete();
    await page().session.confirmDelete();

    requestMade(
      assert,
      'PUT',
      nspaceEndpoint(`/v1/session/destroy/${SESSION}?dc=datacenter&ns=@!namespace`, nspace)
    );
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/key/edit'));
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-success');
  });

  nspaceScenario(
    'Scenario: Invalidating a lock session and receiving an error',
    async function (assert, nspace) {
      respondWith(`/v1/session/destroy/${SESSION}`, { status: 500 });
      await visitKV(assert, nspace);

      await page().session.delete();
      await page().session.confirmDelete();

      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/key/edit'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
    }
  );
});
