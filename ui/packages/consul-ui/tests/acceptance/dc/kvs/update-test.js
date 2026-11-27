/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { click, fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  submit,
  currentURL,
  lastNthRequest,
  requestMadeWithNoBody,
} from 'consul-ui/tests/helpers/acceptance';

// `a PUT request was made to "..." with the body "..."`
const assertPutWithBody = function (assert, url, body) {
  const request = lastNthRequest(null, 'PUT').find((item) => item.url === url);
  assert.ok(request, `Expected a PUT request to ${url}`);
  if (request) {
    assert.strictEqual(request.requestBody, body, 'with the expected body');
  }
};

const visitEdit = async function (assert, nspace, key, encoded = key) {
  await visit('kv', { dc: 'datacenter', kv: key }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/kv/${encoded}/edit`));
};

// Turn the code editor off so the value can be filled as a plain textarea.
const fillInValue = async function (value) {
  await click('[name=json]');
  await fillIn('[name="value"]', value);
};

const assertSaved = function (assert) {
  assert.dom('[data-notification]').hasClass('hds-toast');
  assert.dom('[data-notification]').hasClass('hds-alert--color-success');
};

module('Acceptance | dc / kvs / update: KV Update', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'datacenter');
  });

  [
    { name: 'key', encoded: 'key', value: 'value' },
    { name: '#key', encoded: '%23key', value: 'value' },
    { name: 'key-name', encoded: 'key-name', value: 'a value' },
    { name: 'key name', encoded: 'key%20name', value: 'a value' },
    { name: 'folder/key-name', encoded: 'folder/key-name', value: 'a value' },
  ].forEach(({ name, encoded, value }) => {
    nspaceScenario(
      `Scenario: Update to ${name} change value to ${value}`,
      async function (assert, nspace) {
        createModels('kv', 1, { Key: name, Flags: 12 });

        await visitEdit(assert, nspace, name, encoded);
        assert.equal(document.title, 'Edit Key/Value - Consul');
        await fillInValue(value);
        await submit();

        assertPutWithBody(
          assert,
          nspaceEndpoint(`/v1/kv/${encoded}?dc=datacenter&ns=@!namespace&flags=12`, nspace),
          value
        );
        assertSaved(assert);
      }
    );
  });

  nspaceScenario(
    "Scenario: Update to a key change value to '   '",
    async function (assert, nspace) {
      createModels('kv', 1, { Key: 'key', Flags: 12 });

      await visitEdit(assert, nspace, 'key');
      await fillInValue('   ');
      await submit();

      assertPutWithBody(
        assert,
        nspaceEndpoint('/v1/kv/key?dc=datacenter&ns=@!namespace&flags=12', nspace),
        '   '
      );
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv'));
      assert.equal(document.title, 'Key/Value - Consul');
      assertSaved(assert);
    }
  );

  nspaceScenario("Scenario: Update to a key change value to ''", async function (assert, nspace) {
    createModels('kv', 1, { Key: 'key', Flags: 12 });

    await visitEdit(assert, nspace, 'key');
    await fillInValue('');
    await submit();

    requestMadeWithNoBody(
      assert,
      'PUT',
      nspaceEndpoint('/v1/kv/key?dc=datacenter&ns=@!namespace&flags=12', nspace)
    );
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv'));
    assertSaved(assert);
  });

  nspaceScenario(
    'Scenario: Update to a key when the value is empty',
    async function (assert, nspace) {
      createModels('kv', 1, { Key: 'key', Value: null, Flags: 12 });

      await visitEdit(assert, nspace, 'key');
      await submit();

      requestMadeWithNoBody(
        assert,
        'PUT',
        nspaceEndpoint('/v1/kv/key?dc=datacenter&ns=@!namespace&flags=12', nspace)
      );
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv'));
      assertSaved(assert);
    }
  );

  nspaceScenario('Scenario: There was an error saving the key', async function (assert, nspace) {
    await visitEdit(assert, nspace, 'key');

    respondWith('/v1/kv/key', { status: 500 });
    await submit();

    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/key/edit'));
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
  });
});
