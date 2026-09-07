/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/intentions/create.feature.
//
// Reference example for migrating a `.feature` file off yadda. It uses the
// shared harness in tests/helpers/acceptance.js, which reproduces the
// api-double lifecycle, the CONSUL_NSPACES_ENABLED namespace matrix and the
// request-history assertions the yadda runner previously provided.

import { module } from 'qunit';
import { click, typeIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  submit,
  currentURL,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const services = [
  { Name: 'web', Kind: null },
  { Name: 'db', Kind: null },
  { Name: 'cache', Kind: null },
];

// Prefix a URL with the namespace segment the same way the app's location
// service does (mirrors the yadda `url` dictionary converter).
const withNspace = (nspace, url) =>
  nspace !== '' && typeof nspace !== 'undefined' ? `/~${nspace}${url}` : url;

module('Acceptance | dc / intentions / create: Intention Create', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: with namespaces enabled',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'datacenter');
      api.server.createList('service', 3, services);
      api.server.createList('nspace', 1, [{ Name: 'nspace-0' }]);

      await visit('intention', { dc: 'datacenter' }, { nspace });

      assert.equal(currentURL(), withNspace(nspace, '/datacenter/intentions/create'));
      assert.equal(document.title, 'New Intention - Consul');

      // Set source
      await click('[data-test-source-element] .ember-power-select-trigger');
      await typeIn('.ember-power-select-search-input', 'web');
      await click('.ember-power-select-option:first-child');
      assert
        .dom('[data-test-source-element] .ember-power-select-selected-item')
        .includesText('web');

      // Set destination
      await click('[data-test-destination-element] .ember-power-select-trigger');
      await typeIn('.ember-power-select-search-input', 'db');
      await click('.ember-power-select-option:first-child');
      assert
        .dom('[data-test-destination-element] .ember-power-select-selected-item')
        .includesText('db');

      // Set source nspace
      await click('[data-test-source-nspace] .ember-power-select-trigger');
      await click('.ember-power-select-option:last-child');
      assert
        .dom('[data-test-source-nspace] .ember-power-select-selected-item')
        .includesText('nspace-0');

      // Set destination nspace
      await click('[data-test-destination-nspace] .ember-power-select-trigger');
      await click('.ember-power-select-option:last-child');
      assert
        .dom('[data-test-destination-nspace] .ember-power-select-selected-item')
        .includesText('nspace-0');

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

      assert.equal(currentURL(), withNspace(nspace, '/datacenter/intentions'));
      assert.equal(document.title, 'Intentions - Consul');
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { onlyNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: with namespaces disabled',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'datacenter');
      api.server.createList('service', 3, services);

      await visit('intention', { dc: 'datacenter' }, { nspace });

      assert.equal(currentURL(), withNspace(nspace, '/datacenter/intentions/create'));
      assert.equal(document.title, 'New Intention - Consul');

      // Set source
      await click('[data-test-source-element] .ember-power-select-trigger');
      await typeIn('.ember-power-select-search-input', 'web');
      await click('.ember-power-select-option:first-child');
      assert
        .dom('[data-test-source-element] .ember-power-select-selected-item')
        .includesText('web');

      // Set destination
      await click('[data-test-destination-element] .ember-power-select-trigger');
      await typeIn('.ember-power-select-search-input', 'db');
      await click('.ember-power-select-option:first-child');
      assert
        .dom('[data-test-destination-element] .ember-power-select-selected-item')
        .includesText('db');

      // Specifically set deny
      await click('.value-deny');
      await submit();

      requestMade(
        assert,
        'PUT',
        '/v1/connect/intentions/exact?source=default%2Fdefault%2Fweb&destination=default%2Fdefault%2Fdb&dc=datacenter',
        {
          body: {
            SourceName: 'web',
            DestinationName: 'db',
            Action: 'deny',
          },
        }
      );

      assert.equal(currentURL(), withNspace(nspace, '/datacenter/intentions'));
      assert.equal(document.title, 'Intentions - Consul');
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { notNamespaceable: true }
  );
});
