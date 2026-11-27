/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { click, fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / intentions / form-select: Intention Service Select Dropdowns',
  function (hooks) {
    setupAcceptanceTest(hooks);

    ['source', 'destination'].forEach((name) => {
      nspaceScenario(
        `Scenario: Selecting a future Consul Service in to ${name}`,
        async function (assert, nspace) {
          createModels('dc', 1, 'datacenter');

          await visit('intention', { dc: 'datacenter' }, { nspace });
          assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/intentions/create'));

          await click(`[data-test-${name}-element] .ember-power-select-trigger`);
          await fillIn('.ember-power-select-search-input', 'something');
          await click('.ember-power-select-option:first-child');
          assert
            .dom(`[data-test-${name}-element] .ember-power-select-selected-item`)
            .includesText('something');
        }
      );
    });
  }
);
