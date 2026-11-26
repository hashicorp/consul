/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/show-with-slashes.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / services / show-with-slashes: Show Service that has slashes in its name',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: Given a service with slashes in its name',
      async function (assert, nspace) {
        createModels('dc', 1, 'dc1');
        createModels('node', 1);
        createModels('service', 1, [{ Name: 'hashicorp/service/service-0' }]);

        await visit('services', { dc: 'dc1' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, '/dc1/services'));
        assertVisibleCount(assert, page().services, 1, 'service models');

        await page().services.objectAt(0).service();
        assert.equal(
          currentURL(),
          nspaceURL(nspace, '/:billing/dc1/services/hashicorp%2Fservice%2Fservice-0/instances')
        );
      }
    );
  }
);
