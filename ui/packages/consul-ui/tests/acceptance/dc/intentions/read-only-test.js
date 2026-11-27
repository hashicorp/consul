/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / intentions / read-only', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Viewing a readonly intention', async function (assert, nspace) {
    createModels('dc', 1, 'dc1');
    createModels('intention', 1, { Meta: { 'external-source': 'kubernetes' } });

    await visit(
      'intention',
      { dc: 'dc1', intention: 'default:external-source:web:default:external-source:db' },
      { nspace }
    );
    assert.dom('[data-test-readonly]').exists();
  });
});
