/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import Service from '@ember/service';
import sinon from 'sinon';
import { EnvStub } from 'consul-ui/services/env';

const userDismissedBannerStub = sinon.stub();
const dismissHcpLinkBannerStub = sinon.stub();
const bannerSelector = '[data-test-link-to-hcp-banner]';
module('Integration | Component | link-to-hcp-banner', function (hooks) {
  setupRenderingTest(hooks);

  class HcpLinkStatusStub extends Service {
    get shouldDisplayBanner() {
      return true;
    }
    userDismissedBanner = userDismissedBannerStub;
    dismissHcpLinkBanner = dismissHcpLinkBannerStub;
  }

  hooks.beforeEach(function () {
    this.owner.register('service:hcp-link-status', HcpLinkStatusStub);
  });

  test('it renders banner when hcp-link-status says it should', async function (assert) {
    this.linkData = { isLinked: false };
    await render(hbs`<LinkToHcpBanner @linkData={{this.linkData}} />`);

    assert.dom(bannerSelector).exists({ count: 1 });
    await click(`${bannerSelector} button[aria-label="Dismiss"]`);
    assert.ok(dismissHcpLinkBannerStub.calledOnce, 'userDismissedBanner was called');
    // Can't test that banner is no longer visible since service isn't hooked up
    assert
      .dom('[data-test-link-to-hcp-banner-title]')
      .hasText(
        'Link this cluster to HCP Consul Central in a few steps to start managing your clusters in one place'
      );
    assert
      .dom('[data-test-link-to-hcp-banner-description]')
      .hasText(
        'By linking your clusters to HCP Consul Central, you’ll get global, cross-cluster metrics, visual service maps, and a global API. Link to access a free 90 day trial for full feature access in your HCP organization.'
      );
  });

  test('banner does not render when hcp-link-status says it should NOT', async function (assert) {
    class HcpLinkStatusStub extends Service {
      get shouldDisplayBanner() {
        return false;
      }
      dismissHcpLinkBanner = sinon.stub();
    }
    this.owner.register('service:hcp-link-status', HcpLinkStatusStub);
    this.linkData = { isLinked: false };
    await render(hbs`<LinkToHcpBanner @linkData={{this.linkData}} />`);
    assert.dom(bannerSelector).doesNotExist();
  });

  test('banner does not render when cluster is already linked', async function (assert) {
    class HcpLinkStatusStub extends Service {
      get shouldDisplayBanner() {
        return true;
      }
      dismissHcpLinkBanner = sinon.stub();
    }
    this.owner.register('service:hcp-link-status', HcpLinkStatusStub);
    this.linkData = { isLinked: true };
    await render(hbs`<LinkToHcpBanner @linkData={{this.linkData}} />`);
    assert.dom(bannerSelector).doesNotExist();
  });

  test('banner does not render when we have no cluster link status info', async function (assert) {
    class HcpLinkStatusStub extends Service {
      get shouldDisplayBanner() {
        return true;
      }
      dismissHcpLinkBanner = sinon.stub();
    }
    this.owner.register('service:hcp-link-status', HcpLinkStatusStub);
    this.linkData = undefined;
    await render(hbs`<LinkToHcpBanner @linkData={{this.linkData}} />`);
    assert.dom(bannerSelector).doesNotExist();
  });

  test('it displays different banner text when consul is enterprise', async function (assert) {
    this.owner.register(
      'service:env',
      class Stub extends EnvStub {
        stubEnv = {};
        isEnterprise = true;
      }
    );
    this.linkData = { isLinked: false };

    await render(hbs`<LinkToHcpBanner @linkData={{this.linkData}} />`);
    assert
      .dom('[data-test-link-to-hcp-banner-description]')
      .hasText(
        'By linking your clusters to HCP Consul Central, you’ll get global, cross-cluster metrics, visual service maps, and a global API. HCP Consul Central’s full feature set is included with an Enterprise license.'
      );
  });
});
