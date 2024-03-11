/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { click, visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import HcpLinkModalService from 'consul-ui/services/hcp-link-modal';

const bannerSelector = '[data-test-link-to-hcp-banner]';
const linkToHcpSelector = '[data-test-link-to-hcp]';
const linkToHcpBannerButtonSelector = '[data-test-link-to-hcp-banner-button]';
const linkToHcpModalSelector = '[data-test-link-to-hcp-modal]';
const linkToHcpModalCancelButtonSelector = '[data-test-link-to-hcp-modal-cancel-button]';
module.skip('Acceptance | link to hcp', function (hooks) {
  setupApplicationTest(hooks);
  const correctResourceId =
    'organization/b4432207-bb9c-438e-a160-b98923efa979/project/4b09958c-fa91-43ab-8029-eb28d8cee9d4/hashicorp.consul.global-network-manager.cluster/test-from-api';

  hooks.beforeEach(function () {
    // clear local storage so we don't have any settings
    window.localStorage.clear();
    this.owner.register(
      'service:hcp-link-modal',
      class extends HcpLinkModalService {
        setResourceId(resourceId) {
          super.setResourceId(correctResourceId);
        }
      }
    );
  });

  test('the banner and nav item are initially displayed on services page', async function (assert) {
    // default route is services page so we're good here
    await visit('/');
    // Expect the banner to be visible by default
    assert.dom(bannerSelector).isVisible('Banner is visible by default');
    // expect linkToHCP nav item to be visible as well
    assert.dom(linkToHcpSelector).isVisible('Link to HCP nav item is visible by default');
    // Click on the dismiss button
    await click(`${bannerSelector} button[aria-label="Dismiss"]`);
    assert.dom(bannerSelector).doesNotExist('Banner is gone after dismissing');
    // link to HCP nav item still there
    assert.dom(linkToHcpSelector).isVisible('Link to HCP nav item is visible by default');
    // Refresh the page
    await visit('/');
    assert.dom(bannerSelector).doesNotExist('Banner is still gone after refresh');
    // link to HCP nav item still there
    assert.dom(linkToHcpSelector).isVisible('Link to HCP nav item is visible by default');
  });

  test('the link to hcp modal window appears when trigger from side-nav item and from banner', async function (assert) {
    // default route is services page so we're good here
    await visit('/');
    // Expect the banner to be visible by default
    assert.dom(bannerSelector).isVisible('Banner is visible by default');
    // expect linkToHCP nav item to be visible as well
    assert.dom(linkToHcpSelector).isVisible('Link to HCP nav item is visible by default');
    // Click on the link to HCP banner button
    await click(`${bannerSelector} ${linkToHcpBannerButtonSelector}`);

    // link to HCP modal appears
    assert.dom(linkToHcpModalSelector).isVisible('Link to HCP modal is visible');
    // Click on the cancel button
    await click(`${linkToHcpModalSelector} ${linkToHcpModalCancelButtonSelector}`);
    assert.dom(linkToHcpModalSelector).doesNotExist('Link to HCP modal is gone after cancel');

    // Click on the link to HCP nav item
    await click(`${linkToHcpSelector} button`);

    // link to HCP modal appears
    assert.dom(linkToHcpModalSelector).isVisible('Link to HCP modal is visible');
  });
});
