/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { click, visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';

const bannerSelector = '[data-test-link-to-hcp-banner]';
const linkToHcpSelector = '[data-test-link-to-hcp]';
const linkToHcpNewBadgeSelector = '[data-test-link-to-hcp-new-badge]';
module('Acceptance | link to hcp', function (hooks) {
  setupApplicationTest(hooks);

  hooks.beforeEach(function () {
    // clear local storage so we don't have any settings
    window.localStorage.clear();
  });

  test('the banner and nav item are initially displayed on services page', async function (assert) {
    // default route is services page so we're good here
    await visit('/');
    // Expect the banner to be visible by default
    assert.dom(bannerSelector).isVisible('Banner is visible by default');
    // expect linkToHCP nav item to be visible as well
    assert.dom(linkToHcpSelector).isVisible('Link to HCP nav item is visible by default');
    assert.dom(linkToHcpNewBadgeSelector).isVisible('New badge is visible by default');
    // Click on the dismiss button
    await click(`${bannerSelector} button[aria-label="Dismiss"]`);
    assert.dom(bannerSelector).doesNotExist('Banner is gone after dismissing');
    // link to HCP nav item still there
    assert.dom(linkToHcpSelector).isVisible('Link to HCP nav item is visible by default');
    // NEW Badge is gone
    assert.dom(linkToHcpNewBadgeSelector).doesNotExist('New badge is visible by default');
    // Refresh the page
    await visit('/');
    assert.dom(bannerSelector).doesNotExist('Banner is still gone after refresh');
    // link to HCP nav item still there
    assert.dom(linkToHcpSelector).isVisible('Link to HCP nav item is visible by default');
    // New Badge is gone
    assert.dom(linkToHcpNewBadgeSelector).doesNotExist('New badge is visible by default');
  });
});
