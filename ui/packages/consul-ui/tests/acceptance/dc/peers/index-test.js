/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  disableACLs,
  visit,
  page,
  currentURL,
  assertVisibleCount,
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

// Background: the peers list with 3 peers.
const visitPeers = async function (assert, nspace) {
  createModels('dc', 1, 'dc-1');
  createModels('peer', 3, [{ Name: 'z-peer' }, { Name: 'b-peer' }, { Name: 'a-peer' }]);

  await visit('peers', { dc: 'dc-1' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers'));
  assert.equal(document.title, 'Peers - Consul');
};

module('Acceptance | dc / peers / index: Peers List', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Viewing peers', async function (assert, nspace) {
    await visitPeers(assert, nspace);
    assertVisibleCount(assert, page().peers, 3, 'peer models');
  });

  nspaceScenario('Scenario: Sorting peers', async function (assert, nspace) {
    await visitPeers(assert, nspace);
    const ascending = ['a-peer', 'b-peer', 'z-peer'];
    assertPropertiesVertically(assert, page().peers, 'name', ascending, 'peers');

    await page().sort.name();
    assertPropertiesVertically(
      assert,
      page().peers,
      'name',
      ['z-peer', 'b-peer', 'a-peer'],
      'peers'
    );

    await page().sort.name();
    assertPropertiesVertically(assert, page().peers, 'name', ascending, 'peers');
  });

  nspaceScenario('Scenario: Searching peers', async function (assert, nspace) {
    await visitPeers(assert, nspace);

    await fillIn('[name="search"]', 'a-peer');
    assertVisibleCount(assert, page().peers, 1, 'peer models');
    const named = page().peers.filter((item) => item.isVisible && item.name === 'a-peer');
    assert.equal(named.length, 1, 'sees 1 peer named a-peer');
  });

  nspaceScenario('Scenario: Empty state searching peers', async function (assert, nspace) {
    await visitPeers(assert, nspace);

    await fillIn('[name="search"]', 'no-match');
    assertVisibleCount(assert, page().peers, 0, 'peer models');
    assert.dom('.empty-state h2').includesText('No peers found');
    assert
      .dom('.empty-state p')
      .includesText(
        'No peers were found matching that search, or you may not have the peering:read permissions to access this view.'
      );
    assert.dom('[data-test-empty-state-login]').exists();
  });

  nspaceScenario(
    'Scenario: Empty state searching peers with ACLs disabled',
    async function (assert, nspace) {
      await visitPeers(assert, nspace);
      disableACLs();

      await fillIn('[name="search"]', 'no-match');
      assertVisibleCount(assert, page().peers, 0, 'peer models');
      assert.dom('.empty-state h2').includesText('No peers found');
      assert.dom('.empty-state p').includesText('No peers were found matching that search.');
    }
  );
});
