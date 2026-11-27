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

module('Acceptance | dc / services / show / dc-switch : Switching Datacenters', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Seeing all services when switching datacenters',
    async function (assert, nspace) {
      createModels('dc', 2, ['dc-1', 'dc-2']);
      createModels('node', 1);
      createModels('service', 1, [{ Service: { Service: 'consul', Kind: null } }]);

      await visit('service', { dc: 'dc-1', service: 'consul' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services/consul/topology'));

      await page().navigation.dc();
      await page().navigation.dcs.objectAt(1).name();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-2/services/consul/topology'));
    }
  );
});
