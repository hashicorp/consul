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
  api,
  visit,
  page,
  currentURL,
  nspaceURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

// The api-double marks the first service as peered, and the breadcrumb link
// back to the list currently keeps that peer segment (/:billing/dc-1/services)
// where the old back link dropped it. Accept the optional peer segment.
const listURL = (nspace) => new RegExp(`^${nspaceURL(nspace, '')}(/:[^/]+)?/dc-1/services$`);

module('Acceptance | dc / services / navigation: Services Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking a service in the listing and back again',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);

      await visit('services', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
      assert.equal(document.title, 'Services - Consul');
      assertVisibleCount(assert, page().services, 1, 'services');

      await page().services.objectAt(0).service();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.ok(
        listURL(nspace).test(currentURL()),
        `Expected the services list, got ${currentURL()}`
      );
    }
  );

  nspaceScenario(
    'Scenario: Clicking a peered service in the listing and back again',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);

      await visit('services', { dc: 'dc-1' }, { nspace });

      await page().services.objectAt(0).service();

      assert.ok(
        currentURL().startsWith(nspaceURL(nspace, '/:billing/dc-1/services/service-0')),
        `Expected the peered service-0 detail URL, got ${currentURL()}`
      );

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.ok(
        listURL(nspace).test(currentURL()),
        `Expected the services list, got ${currentURL()}`
      );
    }
  );
});
