/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

const BASE = '/dc1/services/service-0/instances/node-0/service-0-with-id';

const visitInstance = async function (assert, nspace) {
  await visit(
    'instance',
    { dc: 'dc1', service: 'service-0', node: 'node-0', id: 'service-0-with-id' },
    { nspace }
  );
  assert.equal(currentURL(), nspaceURL(nspace, `${BASE}/health-checks`));
};

module('Acceptance | dc / services / instances / Upstreams', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
  });

  nspaceScenario(
    'Scenario: A Service instance without a Proxy does not display Upstreams tab',
    async function (assert, nspace) {
      createModels('proxy', 1, [
        { ServiceProxy: { DestinationServiceName: 'service-1', DestinationServiceID: null } },
      ]);

      await visitInstance(assert, nspace);
      assert.dom('[data-test-tab="tab_upstreams"]').doesNotExist('the tab is not shown');
    }
  );

  nspaceScenario(
    'Scenario: A Service instance with a Proxy displays Upstreams tab',
    async function (assert, nspace) {
      createModels('proxy', 1, [
        { ServiceProxy: { DestinationServiceName: 'service-0', DestinationServiceID: null } },
      ]);

      await visitInstance(assert, nspace);
      const { tabs } = page();
      assert.ok(tabs.upstreamsIsVisible, 'the tab is shown');

      await tabs.upstreams();
      assert.equal(currentURL(), nspaceURL(nspace, `${BASE}/upstreams`));
      assert.ok(tabs.upstreamsIsSelected, 'the tab is selected');
    }
  );
});
