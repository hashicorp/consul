/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/kvs/create.feature.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  visit,
  page,
  submit,
  currentURL,
  fillInFields,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

// The yadda `pause for 200` step.
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const visitCreate = async function (assert, nspace) {
  createModels('dc', 1, 'datacenter');
  await visit('kv', { dc: 'datacenter' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/create'));
  assert.equal(document.title, 'New Key/Value - Consul');
};

module('Acceptance | dc / kvs / create', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Creating a root KV', async function (assert, nspace) {
    await visitCreate(assert, nspace);
    await pause(200);

    await fillInFields({ additional: 'key-value', value: 'value' });
    await submit();

    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv'));
    requestMade(
      assert,
      'PUT',
      nspaceEndpoint('/v1/kv/key-value?dc=datacenter&ns=@namespace', nspace)
    );
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-success');
  });

  nspaceScenario('Scenario: Creating a folder', async function (assert, nspace) {
    await visitCreate(assert, nspace);

    await fillInFields({ additional: 'key-value/' });
    await submit();

    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv'));
    requestMade(
      assert,
      'PUT',
      nspaceEndpoint('/v1/kv/key-value/?dc=datacenter&ns=@namespace', nspace)
    );
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-success');
  });

  nspaceScenario('Scenario: Clicking create from within a folder', async function (assert, nspace) {
    createModels('dc', 1, 'datacenter');
    createModels('kv', 1, ['key-value/']);

    await visit('kvs', { dc: 'datacenter' }, { nspace });
    // folders expand in place, so Create inside one comes from its row menu
    const folder = page().kvs.objectAt(0);
    await folder.actions();
    await pause(200);
    await folder.createInFolder();
    assert.dom('h1').includesText('New Key/Value');
    assert.dom('[data-test-kv-key]').exists();

    await fillInFields({ additional: 'sub-key', value: 'value' });
    // the current page object is still the list, so submit via the DOM
    await click('main [type=submit]');
    requestMade(
      assert,
      'PUT',
      nspaceEndpoint('/v1/kv/key-value/sub-key?dc=datacenter&ns=@namespace', nspace)
    );
  });

  nspaceScenario(
    'Scenario: Clicking create from within a just created folder',
    async function (assert, nspace) {
      await visitCreate(assert, nspace);

      await fillInFields({ additional: 'key-value/' });
      createModels('kv', 1, ['key-value/']);
      await submit();
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv'));

      await click('[data-test-actions-menu]');
      await pause(200);
      await click('[data-test-create-in-folder]');
      assert.dom('h1').includesText('New Key/Value');
      assert.dom('[data-test-breadcrumbs] li').includesText('Key/Value / key-value');
      assert.dom('[data-test-kv-key]').exists();
    }
  );
});
