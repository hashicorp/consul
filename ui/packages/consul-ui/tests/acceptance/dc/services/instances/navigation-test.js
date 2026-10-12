/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/instances/navigation.feature

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / services / instances / navigation: Instance Navigation',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: Clicking an instance in the listing and back again',
      async function (assert, nspace) {
        api.server.createList('dc', 1, 'dc-1');
        // createList('service', 1) populates /v1/internal/ui/services (the list page).
        // createList('instance', 2) sets CONSUL_NODE_COUNT=2, which makes
        // /v1/health/service/:service return 2 instances via the default fixture.
        // No custom override needed — the default fixture shapes work correctly.
        api.server.createList('service', 1);
        api.server.createList('instance', 2);

        await visit('service', { dc: 'dc-1', service: 'service-0' }, { nspace });

        // The instances tab is always present (instances=true in the template).
        // Navigate to it.
        await page().tabs.instances();

        assert.ok(
          currentURL().includes('/dc-1/services/service-0/instances'),
          `Expected instances URL, got ${currentURL()}`
        );

        // Click first instance row → goes to instance detail
        await page().instances.objectAt(0).instance();

        assert.ok(
          currentURL().includes('/dc-1/services/service-0/instances/'),
          `Expected instance detail URL, got ${currentURL()}`
        );

        // Click the first breadcrumb item (Services) to go back to services list
        await click('[data-test-breadcrumb-item]:first-child a');

        assert.ok(
          currentURL().includes('/dc-1/services'),
          `Expected services list URL, got ${currentURL()}`
        );
      },
      { notNamespaceable: true }
    );
  }
);
