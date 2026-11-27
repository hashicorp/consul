/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  page,
  assertProperties,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / nodes / services / list: Node > Services Listing', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Given 1 node', async function (assert, nspace) {
    createModels('dc', 1, 'dc1');
    createModels('node', 1, {
      ID: 'node-0',
      Services: [
        {
          ID: 'service-0-with-id',
          Port: 65535,
          Service: 'service-0',
          Tags: ['monitor', 'two', 'three'],
          Meta: { 'external-source': 'consul' },
        },
        {
          ID: 'service-1',
          Port: 0,
          Service: 'service-1',
          Tags: ['hard drive', 'monitor', 'three'],
          Meta: { 'external-source': 'nomad' },
        },
        {
          ID: 'service-2',
          Port: 1,
          Service: 'service-2',
          Tags: ['one', 'two', 'three'],
          Meta: { 'external-source': 'terraform' },
        },
        {
          ID: 'service-3',
          Port: 2,
          Service: 'service-3',
          Tags: [],
          Meta: { 'external-source': 'kubernetes' },
        },
      ],
    });

    await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });
    await page().tabs.serviceInstances();
    assert.ok(page().tabs.serviceInstancesIsSelected, 'service instances is selected');
    assertProperties(
      assert,
      page().services,
      'externalSource',
      ['consul', 'nomad', 'terraform', 'kubernetes'],
      'services'
    );
  });
});
