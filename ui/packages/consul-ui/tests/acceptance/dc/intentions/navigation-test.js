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
  createModels,
  api,
  visit,
  page,
  pages,
  currentURL,
  nspaceURL,
  nspaceEndpoint,
  assertVisibleCount,
  requestMade,
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
      createModels('dc', 1, 'dc-1');
      api.server.createList('intention', 3, INTENTIONS);

      await visit('intentions', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      assert.equal(document.title, 'Intentions - Consul');
      assertVisibleCount(assert, page().intentionList.intentions, 3, 'intentions');

      // The detail page fetches a single intention.
      api.server.createList('intention', 1, { ID: INTENTIONS[0].ID });
      await page().intentionList.intentions.objectAt(0).intention();
      requestMade(assert, 'GET', nspaceEndpoint('/v1/internal/ui/services?dc=dc-1&ns=*', nspace));

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
    }
  );

  nspaceScenario(
    'Scenario: Clicking the create button and back again',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      api.server.createList('intention', 3, INTENTIONS);

      await visit('intentions', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
      assert.equal(document.title, 'Intentions - Consul');
      assertVisibleCount(assert, page().intentionList.intentions, 3, 'intentions');

      await pages.intentions.create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions/create'));

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/intentions'));
    }
  );
});
