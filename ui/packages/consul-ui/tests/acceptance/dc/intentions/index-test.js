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
  setPermissions,
  setLatency,
  disableACLs,
  visit,
  page,
  currentURL,
  assertVisibleCount,
  waitForVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

const intentions = () => page().intentionList.intentions;

module('Acceptance | dc / intentions / index', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Viewing intentions in the listing', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('intention', 3);

    await visit('intentions', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
    assert.equal(document.title, 'Intentions - Consul');
    assertVisibleCount(assert, intentions(), 3, 'intention models');
  });

  nspaceScenario(
    'Scenario: Viewing intentions with no write access',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('intention', 3);
      setPermissions({ intention: { write: false } });

      await visit('intentions', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      // Intention write is currently hardcoded to true until the API does what
      // we need, so the create button is still shown. Once that changes this
      // should assert that it isn't.
      assert.dom('[data-test-create]').exists();
    }
  );

  nspaceScenario(
    'Scenario: Viewing intentions in the listing live updates',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('intention', 3);
      setLatency(100);

      await visit('intentions', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      await waitForVisibleCount(assert, intentions(), 3, 'intention models');

      createModels('intention', 5);
      await waitForVisibleCount(assert, intentions(), 5, 'intention models');
      createModels('intention', 1);
      await waitForVisibleCount(assert, intentions(), 1, 'intention models');
      createModels('intention', 0);
      await waitForVisibleCount(assert, intentions(), 0, 'intention models');
    }
  );

  nspaceScenario(
    'Scenario: Viewing intentions in the listing with CRDs',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('intention', 1, { Meta: { 'external-source': 'kubernetes' } });

      await visit('intentions', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      assert.ok(page().intentionList.customResourceNotice, 'shows the CRD notice');
    }
  );

  nspaceScenario(
    'Scenario: Viewing intentions in the listing without CRDs',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('intention', 1, { Meta: { 'external-source': 'consul' } });

      await visit('intentions', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      assert.notOk(page().intentionList.customResourceNotice, 'shows no CRD notice');
    }
  );

  nspaceScenario(
    'Scenario: Viewing an empty intentions page with acl enabled',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('intention', 0);

      await visit('intentions', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      assert.equal(document.title, 'Intentions - Consul');
      assertVisibleCount(assert, intentions(), 0, 'intention models');
      assert
        .dom('.empty-state p')
        .includesText(
          "There don't seem to be any Intentions in this Consul cluster, or you may not have intentions:read permissions access to this view."
        );
      assert.dom('[data-test-empty-state-login]').exists();
    }
  );

  nspaceScenario(
    'Scenario: Viewing an empty intentions page with acl disabled',
    async function (assert, nspace) {
      disableACLs();
      createModels('dc', 1, 'dc-1');
      createModels('intention', 0);

      await visit('intentions', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      assert.equal(document.title, 'Intentions - Consul');
      assertVisibleCount(assert, intentions(), 0, 'intention models');
      assert
        .dom('.empty-state p')
        .includesText("There don't seem to be any Intentions in this Consul cluster.");
      assert.dom('[data-test-empty-state-login]').doesNotExist();
    }
  );
});
