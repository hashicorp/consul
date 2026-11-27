/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / intentions / filtered-select: Intention Service Select Dropdowns',
  function (hooks) {
    setupAcceptanceTest(hooks);

    ['source', 'destination'].forEach((name) => {
      nspaceScenario(
        `Scenario: Opening the ${name} dropdown with 2 services and 2 proxy services`,
        async function (assert, nspace) {
          createModels('dc', 1, 'datacenter');
          createModels('service', 4, [
            { Name: 'service-0', Kind: null },
            { Name: 'service-1', Kind: null },
            { Name: 'service-2', Kind: 'connect-proxy' },
            { Name: 'service-3', Kind: 'connect-proxy' },
          ]);

          await visit('intention', { dc: 'datacenter' }, { nspace });
          assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/intentions/create'));

          await click(`[data-test-${name}-element] .ember-power-select-trigger`);
          assert
            .dom('.ember-power-select-option:nth-last-child(3)')
            .includesText('* (All Services)');
          assert.dom('.ember-power-select-option:nth-last-child(2)').includesText('service-0');
          assert.dom('.ember-power-select-option:last-child').includesText('service-1');
        }
      );
    });

    ['source', 'destination'].forEach((name) => {
      nspaceScenario(
        `Scenario: Opening the ${name} dropdown with 2 services with the same name from different nspaces`,
        async function (assert, nspace) {
          createModels('dc', 1, 'datacenter');
          createModels('service', 2, [
            { Name: 'service-0', Kind: null },
            { Name: 'service-0', Namespace: 'nspace', Kind: null },
          ]);

          await visit('intention', { dc: 'datacenter' }, { nspace });
          assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/intentions/create'));

          await click(`[data-test-${name}-element] .ember-power-select-trigger`);
          assert
            .dom('.ember-power-select-option:nth-last-child(2)')
            .includesText('* (All Services)');
          assert.dom('.ember-power-select-option:last-child').includesText('service-0');
        }
      );
    });
  }
);
