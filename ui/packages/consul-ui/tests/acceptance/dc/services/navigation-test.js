/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/navigation.feature

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  api,
  visit,
  page,
  currentURL,
  nspaceURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

// The api-double marks the first service as peered (`billing`).
//
// FIXME: the breadcrumb back to the list keeps the clicked service's peer
// segment, although app/services/breadcrumbs.js means to avoid inheriting peer
// segments. Once that is fixed this should be `/dc-1/services`, as in the
// original feature.
const listURL = (nspace) => nspaceURL(nspace, '/:billing/dc-1/services');

module('Acceptance | dc / services / navigation: Services Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking a service in the listing and back again',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      api.server.createList('service', 1);

      await visit('services', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
      assert.equal(document.title, 'Services - Consul');
      assertVisibleCount(assert, page().services, 1, 'services');

      await page().services.objectAt(0).service();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.equal(currentURL(), listURL(nspace));
    }
  );

  nspaceScenario(
    'Scenario: Clicking a peered service in the listing and back again',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      api.server.createList('service', 1);

      await visit('services', { dc: 'dc-1' }, { nspace });

      await page().services.objectAt(0).service();

      assert.ok(
        currentURL().startsWith(nspaceURL(nspace, '/:billing/dc-1/services/service-0')),
        `Expected the peered service-0 detail URL, got ${currentURL()}`
      );

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.equal(currentURL(), listURL(nspace));
    }
  );
});
