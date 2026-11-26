/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/show/services.feature.

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

module('Acceptance | dc / services / show / services', function (hooks) {
  setupAcceptanceTest(hooks);

  // Background: a terminating-gateway service.
  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
    createModels('node', 1);
    createModels('service', 1, [
      { Service: { Name: 'terminating-gateway-1', Kind: 'terminating-gateway' } },
    ]);
  });

  nspaceScenario('Scenario: Seeing the Linked Services tab', async function (assert, nspace) {
    await visitService(assert, nspace, 'terminating-gateway-1');
    const { tabs } = page();
    assert.ok(tabs.linkedServicesIsVisible, 'the tab is shown');
    await tabs.linkedServices();
    assert.ok(tabs.linkedServicesIsSelected, 'the tab is selected');
  });

  nspaceScenario('Scenario: Seeing the list of Linked Services', async function (assert, nspace) {
    createModels('service', 3);

    await visitService(assert, nspace, 'terminating-gateway-1');
    const { tabs } = page();
    await tabs.linkedServices();

    assertVisibleCount(assert, tabs.linkedServicesTab.services, 3, 'service models');
  });

  [
    { Name: 'service', Kind: null },
    { Name: 'ingress-gateway', Kind: 'ingress-gateway' },
    { Name: 'api-gateway', Kind: 'api-gateway' },
    { Name: 'mesh-gateway', Kind: 'mesh-gateway' },
  ].forEach(({ Name, Kind }) => {
    nspaceScenario("Scenario: Don't see the Linked Services tab", async function (assert, nspace) {
      createModels('dc', 1, 'dc1');
      createModels('node', 1);
      createModels('service', 1, [{ Service: { Name, Kind } }]);

      await visitService(assert, nspace, Name);
      assert.dom('[data-test-tab="tab_linked-services"]').doesNotExist('the tab is not shown');
    });
  });
});
