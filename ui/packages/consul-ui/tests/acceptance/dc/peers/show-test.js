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

// A page-object property that may not exist.
const isShown = function (get) {
  try {
    return !!get();
  } catch (e) {
    return false;
  }
};

const viewPeerFromList = async function (assert, nspace, PeerID) {
  createModels('dc', 1, 'dc-1');
  createModels('peer', 1, { Name: 'a-peer', State: 'ACTIVE', PeerID });

  await visit('peers', { dc: 'dc-1' }, { nspace });
  const peer = page().peers.objectAt(0);
  await peer.actions();
  await peer.view();
  assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers/a-peer/imported-services'));
};

module('Acceptance | dc / peers / show: Peers show', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Dialer side tabs', async function (assert, nspace) {
    // dialer side
    await viewPeerFromList(assert, nspace, null);

    const { tabs } = page();
    assert.ok(
      isShown(() => tabs.importedServicesIsVisible),
      'shows imported services'
    );
    assert.ok(
      isShown(() => tabs.exportedServicesIsVisible),
      'shows exported services'
    );
    assert.notOk(
      isShown(() => tabs.serverAddressesIsVisible),
      'no server addresses'
    );
  });

  nspaceScenario('Scenario: Receiver side tabs', async function (assert, nspace) {
    // receiver side
    await viewPeerFromList(assert, nspace, 'some-peer');

    const { tabs } = page();
    assert.ok(
      isShown(() => tabs.importedServicesIsVisible),
      'shows imported services'
    );
    assert.ok(
      isShown(() => tabs.exportedServicesIsVisible),
      'shows exported services'
    );
    assert.ok(
      isShown(() => tabs.serverAddressesIsVisible),
      'shows server addresses'
    );
  });

  const visitPeer = function (nspace, method) {
    return visit('peer', { dc: 'dc-1', peer: 'a-peer' }, { nspace, method });
  };

  nspaceScenario('Scenario: Imported Services Empty', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('peer', 1, { Name: 'a-peer', State: 'ACTIVE' });
    createModels('service', 0);

    await visitPeer(nspace, 'visit');
    assert.dom('[data-test-imported-services-empty]').exists();
  });

  nspaceScenario('Scenario: Imported Services not empty', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('peer', 1, { Name: 'a-peer', State: 'ACTIVE' });
    createModels('service', 1, { Name: 'service-for-peer-a' });

    await visitPeer(nspace, 'visit');
    assert.dom('[data-test-imported-services-empty]').doesNotExist();
  });

  nspaceScenario('Scenario: Exported Services Empty', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('peer', 1, { Name: 'a-peer', State: 'ACTIVE' });
    createModels('service', 0);

    await visitPeer(nspace, 'visitExported');
    assert.dom('[data-test-exported-services-empty]').exists();
  });

  nspaceScenario('Scenario: Exported Services not empty', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('peer', 1, { Name: 'a-peer', State: 'ACTIVE' });
    createModels('service', 1, { Name: 'service-for-peer-a' });

    await visitPeer(nspace, 'visitExported');
    assert.dom('[data-test-exported-services-empty]').doesNotExist();
  });

  nspaceScenario('Scenario: Addresses Empty', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('peer', 1, { Name: 'a-peer', State: 'ACTIVE', PeerServerAddresses: null });

    await visitPeer(nspace, 'visitAddresses');
    assert.dom('[data-test-addresses-empty]').exists();
  });

  nspaceScenario('Scenario: Addresses Not Empty', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('peer', 1, { Name: 'a-peer', State: 'ACTIVE' });

    await visitPeer(nspace, 'visitAddresses');
    assert.dom('[data-test-addresses-empty]').doesNotExist();
  });
});
