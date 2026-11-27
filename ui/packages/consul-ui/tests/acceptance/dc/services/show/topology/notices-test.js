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
  setLocalDatacenter,
  setDefaultACLPolicy,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

const db1 = (Intention) => ({
  Name: 'db-1',
  Namespace: 'default',
  Datacenter: 'datacenter',
  Intention,
});
const db2 = {
  Name: 'db-2',
  Namespace: 'default',
  Datacenter: 'datacenter',
  Intention: { Allowed: false },
};
const tproxy = (TransparentProxy) => ({
  FilteredByACLs: false,
  TransparentProxy: false,
  DefaultAllow: false,
  WildcardIntention: false,
  Downstreams: [
    {
      Name: 'db',
      Namespace: 'default',
      Datacenter: 'datacenter',
      Intention: { Allowed: true },
      Source: 'specific-intention',
      TransparentProxy,
    },
  ],
});

const visitTopology = async function (assert, nspace) {
  await visit('service', { dc: 'datacenter', service: 'web' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/services/web/topology'));
};

module('Acceptance | dc / services / show / topology / notices', function (hooks) {
  setupAcceptanceTest(hooks);

  // Background
  hooks.beforeEach(function () {
    createModels('dc', 1, 'datacenter');
    setLocalDatacenter('datacenter');
    createModels('intention', 1, {
      SourceNS: 'default',
      SourceName: 'web',
      DestinationNS: 'default',
      DestinationName: 'db',
      ID: 'intention-id',
    });
    createModels('node', 1);
    createModels('service', 1, [{ Service: { Name: 'web', Kind: null } }]);
  });

  nspaceScenario('Scenario: default ACL policy is set to "allow"', async function (assert, nspace) {
    createModels('topology', 1, {
      FilteredByACLs: false,
      TransparentProxy: false,
      Downstreams: [db1({ Allowed: false })],
      Upstreams: [db2],
    });
    setDefaultACLPolicy('allow');

    await visitTopology(assert, nspace);
    assert.ok(page().tabs.topologyTab.defaultAllowNotice.see, 'shows the default-allow notice');
  });

  nspaceScenario(
    'Scenario: A Downstream service has a wildcard intention',
    async function (assert, nspace) {
      createModels('topology', 1, {
        FilteredByACLs: false,
        TransparentProxy: false,
        Downstreams: [db1({ Allowed: true, HasExact: false })],
        Upstreams: [db2],
      });

      await visitTopology(assert, nspace);
      assert.ok(page().tabs.topologyTab.wildcardIntention.see, 'shows the wildcard notice');
    }
  );

  nspaceScenario('Scenario: Response is filtered by ACLs', async function (assert, nspace) {
    createModels('topology', 1, {
      FilteredByACLs: true,
      TransparentProxy: false,
      Downstreams: [db1({ Allowed: false })],
      Upstreams: [db2],
    });

    await visitTopology(assert, nspace);
    assert.ok(page().tabs.topologyTab.filteredByACLs.see, 'shows the filtered-by-ACLs notice');
  });

  nspaceScenario(
    'Scenario: TProxy for a downstream is set to false and globally false',
    async function (assert, nspace) {
      createModels('topology', 1, tproxy(false));

      await visitTopology(assert, nspace);
      assert.ok(page().tabs.topologyTab.notDefinedIntention.see, 'shows the not-defined notice');
    }
  );

  nspaceScenario(
    'Scenario: TProxy for a downstream is set to true and globally false',
    async function (assert, nspace) {
      createModels('topology', 1, tproxy(true));

      await visitTopology(assert, nspace);
      assert.notOk(page().tabs.topologyTab.notDefinedIntention.see, 'no not-defined notice');
    }
  );
});
