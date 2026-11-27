/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { visit } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  pages,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / show / navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Accessing peered service directly', async function (assert) {
    createModels('dc', 1, 'dc-1');
    createModels('service', 1);

    await visit('/:billing/dc-1/services/service-0');
    assert.strictEqual(pages.service.peer, 'billing');
  });
});
