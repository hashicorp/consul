/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/show/intentions/index.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
  assertVisibleCount,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

// Background: the intentions tab of service-0, with 3 intentions.
const visitIntentions = async function (assert, nspace) {
  createModels('dc', 1, 'dc1');
  createModels('node', 1);
  createModels('service', 1, [
    { Service: { Kind: null, Name: 'service-0', ID: 'service-0-with-id' } },
  ]);
  createModels('intention', 3, [
    {
      ID: '755b72bd-f5ab-4c92-90cc-bed0e7d8e9f0',
      Action: 'allow',
      Meta: null,
      SourceName: 'name',
      DestinationName: 'destination',
      SourceNS: 'default',
      DestinationNS: 'default',
      SourcePartition: 'default',
      DestinationPartition: 'default',
      SourcePeer: '',
    },
    { ID: '755b72bd-f5ab-4c92-90cc-bed0e7d8e9f1', Action: 'deny', Meta: null, SourcePeer: '' },
    { ID: '0755b72bd-f5ab-4c92-90cc-bed0e7d8e9f2', Action: 'deny', Meta: null, SourcePeer: '' },
  ]);

  await visit('service', { dc: 'dc1', service: 'service-0' }, { nspace });
  assert.equal(document.title, 'service-0 - Consul');
  const { tabs } = page();
  assert.ok(tabs.intentionsIsVisible, 'the intentions tab is shown');
  await tabs.intentions();
  assert.ok(tabs.intentionsIsSelected, 'the intentions tab is selected');
};

module(
  'Acceptance | dc / services / show / intentions / index: Intentions per service',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario('Scenario: I can see intentions', async function (assert, nspace) {
      await visitIntentions(assert, nspace);

      const intentions = page().intentionList.intentions;
      assertVisibleCount(assert, intentions, 3, 'intention models');
      await intentions.objectAt(0).intention();
      assert.equal(
        currentURL(),
        nspaceURL(
          nspace,
          '/dc1/services/service-0/intentions/default:default:name:default:default:destination'
        )
      );
    });

    nspaceScenario('Scenario: I can delete intentions', async function (assert, nspace) {
      await visitIntentions(assert, nspace);

      const intention = page().intentionList.intentions.objectAt(0);
      await intention.actions();
      await intention.delete();
      await intention.confirmInlineDelete();

      requestMade(
        assert,
        'DELETE',
        '/v1/connect/intentions/exact?source=default%2Fdefault%2Fname&destination=default%2Fdefault%2Fdestination&dc=dc1'
      );
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    });
  }
);
