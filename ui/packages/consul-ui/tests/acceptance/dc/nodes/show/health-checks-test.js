/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nodes/show/health-checks.feature,
// plus a regression test for the "Check type" sort (see the last scenario).

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

const nodeWithSerfCheck = (status, output) => ({
  ID: 'node-0',
  Checks: [
    {
      Type: '',
      Name: 'Serf Health Status',
      CheckID: 'serfHealth',
      Status: status,
      Output: output,
    },
  ],
});

// The names rendered in the card list, top to bottom.
const renderedCheckNames = () =>
  [...document.querySelectorAll('.consul-health-check-list .health-check-output__name')].map((el) =>
    el.textContent.trim()
  );

module('Acceptance | dc / nodes / show / health-checks', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: A failing serf check', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc1');
    api.server.createList('node', 1, nodeWithSerfCheck('critical', 'ouch'));

    await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc1/nodes/node-0/health-checks'));
    assert.dom('[data-test-tab="tab_health-checks"]').hasClass('selected');
    assert.dom('[data-test-critical-serf-notice]').exists();
  });

  nspaceScenario('Scenario: A passing serf check', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc1');
    api.server.createList('node', 1, nodeWithSerfCheck('passing', 'Agent alive and reachable'));

    await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc1/nodes/node-0/health-checks'));
    assert.dom('[data-test-tab="tab_health-checks"]').hasClass('selected');
    assert.dom('[data-test-critical-serf-notice]').doesNotExist();
  });

  // Regression: the "Check type" sort is presented as "Service to Node"
  // (Kind:asc) / "Node to Service" (Kind:desc). A plain alphabetical Kind sort
  // ordered 'node' before 'service' (n < s) — the reverse of the labels — so
  // picking "Service to Node" actually listed node checks first. Assert the
  // rendered order matches the option's label in both directions.
  nspaceScenario(
    'Scenario: Sorting by check type orders checks to match the option labels',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc1');
      // A service check (non-empty ServiceID -> Kind "service") and a node
      // check (empty ServiceID -> Kind "node").
      api.server.createList('node', 1, {
        ID: 'node-0',
        Checks: [
          {
            Name: 'Node Check',
            CheckID: 'node-check',
            ServiceID: '',
            Node: 'node-0',
            Status: 'passing',
            Type: 'tcp',
            Output: 'output',
          },
          {
            Name: 'Service Check',
            CheckID: 'service-check',
            ServiceID: 'service-1',
            ServiceName: 'service-1',
            Node: 'node-0',
            Status: 'passing',
            Type: 'http',
            Output: 'output',
          },
        ],
      });

      await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });

      // "Service to Node" (Kind:asc): the service check comes first.
      await click('[data-test-sort-control] [data-test-sort-toggle]');
      await click('[data-test-sort-option="Kind:asc"]');
      let names = renderedCheckNames();
      assert.ok(
        names.indexOf('Service Check') < names.indexOf('Node Check'),
        `"Service to Node" should list the service check before the node check (got: ${names.join(
          ', '
        )})`
      );

      // "Node to Service" (Kind:desc): the node check comes first.
      await click('[data-test-sort-control] [data-test-sort-toggle]');
      await click('[data-test-sort-option="Kind:desc"]');
      names = renderedCheckNames();
      assert.ok(
        names.indexOf('Node Check') < names.indexOf('Service Check'),
        `"Node to Service" should list the node check before the service check (got: ${names.join(
          ', '
        )})`
      );
    }
  );
});
