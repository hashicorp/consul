/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/intentions/navigation.feature

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
  pages,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

const INTENTIONS = [
  { ID: '755b72bd-f5ab-4c92-90cc-bed0e7d8e9f0', Action: 'allow', Meta: null, SourcePeer: '' },
  { ID: '755b72bd-f5ab-4c92-90cc-bed0e7d8e9f1', Action: 'deny', Meta: null },
  { ID: '0755b72bd-f5ab-4c92-90cc-bed0e7d8e9f2', Action: 'deny', Meta: null },
];

module('Acceptance | dc / intentions / navigation: Intentions Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking an intention in the listing and back again',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('intention', 3, INTENTIONS);

      await visit('intentions', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      assert.equal(document.title, 'Intentions - Consul');

      await page().intentionList.intentions.objectAt(0).intention();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.ok(
        currentURL().includes('/dc-1/intentions'),
        `Expected URL to contain /dc-1/intentions, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Clicking the create button and back again',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('intention', 3, INTENTIONS);

      await visit('intentions', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));

      await pages.intentions.create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions/create'));

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.ok(
        currentURL().includes('/dc-1/intentions'),
        `Expected URL to contain /dc-1/intentions, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );
});
