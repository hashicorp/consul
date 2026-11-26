/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/show/topology/intentions.feature.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  setLocalDatacenter,
  visit,
} from 'consul-ui/tests/helpers/acceptance';

const allowConnection = async function (nspace) {
  await visit('service', { dc: 'datacenter', service: 'web' }, { nspace });
  await click('.consul-topology-metrics [data-test-action]');
  await click('.consul-topology-metrics [data-test-confirm]');
};

module('Acceptance | dc / services / show / topology / intentions', function (hooks) {
  setupAcceptanceTest(hooks);

  // Background: web has an upstream db with an intention.
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
    createModels('topology', 1, {
      Downstreams: [],
      Upstreams: [{ Name: 'db', Namespace: 'default', Datacenter: 'datacenter', Intention: {} }],
    });
  });

  nspaceScenario(
    'Scenario: Allow a connection between service and upstream by saving an intention',
    async function (assert, nspace) {
      await allowConnection(nspace);
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    }
  );

  nspaceScenario(
    'Scenario: There was an error saving the intention',
    async function (assert, nspace) {
      respondWith('/v1/connect/intentions/exact', { status: 500 });
      await allowConnection(nspace);
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
    }
  );
});
