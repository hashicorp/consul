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
  submit,
  currentURL,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const selectService = async function (assert, side, name) {
  await click(`[data-test-${side}-element] .ember-power-select-trigger`);
  await fillIn('.ember-power-select-search-input', name);
  await click('.ember-power-select-option:first-child');
  assert.dom(`[data-test-${side}-element] .ember-power-select-selected-item`).includesText(name);
};

const selectLastNspace = async function (assert, side) {
  await click(`[data-test-${side}-nspace] .ember-power-select-trigger`);
  await click('.ember-power-select-option:last-child');
  assert
    .dom(`[data-test-${side}-nspace] .ember-power-select-selected-item`)
    .includesText('nspace-0');
};

const assertSaved = function (assert, nspace) {
  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/services/db/intentions'));
  assert.dom('[data-notification]').hasClass('hds-toast');
  assert.dom('[data-notification]').hasClass('hds-alert--color-success');
};

module(
  'Acceptance | dc / services / intentions / create: Intention Create per Service',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: with namespaces enabled',
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');
        createModels('service', 3, [
          { Name: 'web', Kind: null },
          { Name: 'db', Kind: null },
          { Name: 'cache', Kind: null },
        ]);
        createModels('instance', 1, [{ Service: { ID: 'db', Service: 'db', Kind: null } }]);
        createModels('nspace', 1, [{ Name: 'nspace-0' }]);

        await visit('intention', { dc: 'datacenter', service: 'db' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/services/db/intentions/create'));

        await selectService(assert, 'source', 'web');
        await selectService(assert, 'destination', 'db');
        await selectLastNspace(assert, 'source');
        await selectLastNspace(assert, 'destination');
        // Specifically set deny
        await click('.value-deny');
        await submit();

        requestMade(
          assert,
          'PUT',
          '/v1/connect/intentions/exact?source=default%2Fnspace-0%2Fweb&destination=default%2Fnspace-0%2Fdb&dc=datacenter',
          {
            body: {
              SourceName: 'web',
              DestinationName: 'db',
              SourceNS: 'nspace-0',
              DestinationNS: 'nspace-0',
              SourcePartition: 'default',
              DestinationPartition: 'default',
              Action: 'deny',
            },
          }
        );
        assertSaved(assert, nspace);
      },
      { onlyNamespaceable: true }
    );

    nspaceScenario(
      'Scenario: with namespaces disabled',
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');
        createModels('instance', 3, [
          { Service: { ID: 'web', Service: 'web', Kind: null } },
          { Service: { ID: 'db', Service: 'db', Kind: null } },
          { Service: { ID: 'cache', Service: 'cache', Kind: null } },
        ]);

        await visit('intention', { dc: 'datacenter', service: 'db' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/services/db/intentions/create'));

        await selectService(assert, 'source', 'web');
        // Specifically set deny
        await click('.value-deny');
        await submit();

        requestMade(
          assert,
          'PUT',
          '/v1/connect/intentions/exact?source=default%2Fdefault%2Fweb&destination=default%2Fdefault%2Fdb&dc=datacenter',
          { body: { SourceName: 'web', DestinationName: 'db', Action: 'deny' } }
        );
        assertSaved(assert, nspace);
      },
      { notNamespaceable: true }
    );
  }
);
