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

const folders = [
  '__secretzzz/',
  'a-thing-service/',
  'a-thing-y-again-service/',
  'a-thing-y-againzz-service/',
  'a-z-search-service/',
  'blood-pressure-service/',
  'callToAction-items/',
  'configuration/',
  'content-service/',
  'currentRepository-jobs/',
  'currentRepository-service/',
  'first-service/',
  'logs-service/',
  'rabmq-svc/',
  'rabmqUtilities/',
  'schedule-service/',
  'vanApp-service/',
  'vanCat-service/',
  'vanTaxi-service/',
];

module('Acceptance | dc / kvs / list-order', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: I have 19 folders', async function (assert, nspace) {
    createModels('dc', 1, 'datacenter');
    createModels('kv', 19, folders);

    await visit('kvs', { dc: 'datacenter' }, { nspace });
    // the first page of the listing, which holds 10 entries
    assertProperties(assert, page().kvs, 'name', folders.slice(0, 10), 'kvs');
  });
});
