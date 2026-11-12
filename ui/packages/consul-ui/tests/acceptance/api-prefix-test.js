/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/api-prefix.feature

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  setApiPrefix,
  visit,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | api-prefix', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Requests use the configured API prefix', async function (assert) {
    createModels('dc', 1, 'dc1');
    setApiPrefix('/prefixed-api');

    // The original `I visit the index page` step never applied the namespace.
    await visit('index');

    requestMade(assert, 'GET', '/prefixed-api/v1/catalog/datacenters');
  });
});
