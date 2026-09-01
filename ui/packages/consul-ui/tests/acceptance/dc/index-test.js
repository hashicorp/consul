/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/index.feature.
//
// The original scenario carried `@ignore`, so the runner skipped it. We
// preserve that here with `{ ignore: true }` (which maps to QUnit's `skip`).

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
} from 'consul-ui/tests/helpers/acceptance';

const visible = (collection) => collection.filter((item) => item.isVisible).length;

module('Acceptance | dc / index: Datacenters', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Arriving at the service page',
    async function (assert) {
      api.server.createList('dc', 10);

      await visit('index');
      await click('[data-test-datacenter-selected]');

      assert.equal(visible(page().dcs), 10, 'sees 10 datacenter models');
    },
    { ignore: true }
  );
});
