/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/show/tags.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  page,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / show / tags', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
    createModels('node', 1);
  });

  [
    ['A service with multiple tags', ['tag', 'tag1', 'tag2']],
    ['A service with multiple duplicated tags', ['tag1', 'tag2', 'tag', 'tag', 'tag1', 'tag2']],
  ].forEach(([title, Tags]) => {
    nspaceScenario(`Scenario: ${title}`, async function (assert, nspace) {
      createModels('service', 1, [{ Service: { Name: 'service', Kind: null, Tags } }]);

      await visit('service', { dc: 'dc1', service: 'service' }, { nspace });
      const { tabs } = page();
      assert.ok(tabs.tagsIsVisible, 'the tags tab is shown');
      await tabs.tags();
      assert.ok(tabs.tagsIsSelected, 'the tags tab is selected');
      assertVisibleCount(assert, tabs.tagsTab.tags, 3, 'tag models');
    });
  });
});
