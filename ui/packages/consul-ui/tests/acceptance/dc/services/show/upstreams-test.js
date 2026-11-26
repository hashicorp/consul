/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/show/upstreams.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  page,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

const visitService = async function (assert, nspace, service) {
  await visit('service', { dc: 'dc1', service }, { nspace });
  assert.equal(document.title, `${service} - Consul`);
};

module('Acceptance | dc / services / show / upstreams', function (hooks) {
  setupAcceptanceTest(hooks);

  // Background: a ingress-gateway service.
  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
    createModels('node', 1);
    createModels('service', 1, [
      { Service: { Name: 'ingress-gateway-1', Kind: 'ingress-gateway' } },
    ]);
  });

  nspaceScenario('Scenario: Seeing the Upstreams tab', async function (assert, nspace) {
    await visitService(assert, nspace, 'ingress-gateway-1');
    const { tabs } = page();
    assert.ok(tabs.upstreamsIsVisible, 'the tab is shown');
    await tabs.upstreams();
    assert.ok(tabs.upstreamsIsSelected, 'the tab is selected');
  });

  nspaceScenario('Scenario: Seeing the list of Upstreams', async function (assert, nspace) {
    createModels('service', 3);

    await visitService(assert, nspace, 'ingress-gateway-1');
    const { tabs } = page();
    await tabs.upstreams();
    assert.ok(tabs.upstreamsIsSelected, 'the tab is selected');
    assertVisibleCount(assert, tabs.upstreamsTab.services, 3, 'service models');
  });

  [
    { Name: 'service', Kind: null },
    { Name: 'terminating-gateway', Kind: 'terminating-gateway' },
    { Name: 'mesh-gateway', Kind: 'mesh-gateway' },
  ].forEach(({ Name, Kind }) => {
    nspaceScenario("Scenario: Don't see the Upstreams tab", async function (assert, nspace) {
      createModels('dc', 1, 'dc1');
      createModels('node', 1);
      createModels('service', 1, [{ Service: { Name, Kind } }]);

      await visitService(assert, nspace, Name);
      assert.dom('[data-test-tab="tab_upstreams"]').doesNotExist('the tab is not shown');
    });
  });
});
