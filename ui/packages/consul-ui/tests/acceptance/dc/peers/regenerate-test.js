/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  visit,
  page,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / peers / regenerate: Regenerate Peer Token', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: The dialer can regenerate the peering token',
    async function (assert, nspace) {
      createModels('dc', 1, 'datacenter');
      // dialer does not have a PeerID
      createModels('peer', 1, { Name: 'peer-name', State: 'ACTIVE', PeerID: null });
      respondWith('/v1/peering/token', { body: { PeeringToken: 'an-encoded-token' } });

      await visit('peers', { dc: 'datacenter' }, { nspace });
      const peer = page().peers.objectAt(0);
      await peer.actions();
      await peer.regenerate();
      assert
        .dom('#regenerate-token-modal [data-test-peering-token]')
        .includesText('an-encoded-token');
    }
  );

  nspaceScenario(
    'Scenario: The receiver cannot regenerate the peering token',
    async function (assert, nspace) {
      createModels('dc', 1, 'datacenter');
      // receiver holds a PeerID
      createModels('peer', 1, { Name: 'peer-name', State: 'ACTIVE', PeerID: 'some-id' });

      await visit('peers', { dc: 'datacenter' }, { nspace });
      await page().peers.objectAt(0).actions();
      assert
        .dom('.consul-peer-list [data-test-tabular-row] [data-test-regenerate]')
        .doesNotExist('there is no regenerate action');
    }
  );
});
