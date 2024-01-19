import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import Service from '@ember/service';
import sinon from 'sinon';

const userDismissedBannerStub = sinon.stub();
const userHasLinkedStub = sinon.stub();
const dismissHcpLinkBannerStub = sinon.stub();
const bannerSelector = '[data-test-link-to-hcp-banner]';
module('Integration | Component | link-to-hcp-banner', function (hooks) {
  setupRenderingTest(hooks);

  class HcpLinkStatusStub extends Service {
    get shouldDisplayBanner() {
      return true;
    }
    userDismissedBanner = userDismissedBannerStub;
    userHasLinked = userHasLinkedStub;
    dismissHcpLinkBanner = dismissHcpLinkBannerStub;
  }

  class EnvStub extends Service {
    isEnterprise = false;
    var(key) {
      return key;
    }
  }

  hooks.beforeEach(function () {
    this.owner.register('service:hcp-link-status', HcpLinkStatusStub);
    this.owner.register('service:env', EnvStub);
  });

  test('it renders banner when hcp-link-status says it should', async function (assert) {
    await render(hbs`<LinkToHcpBanner />`);

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
      userDismissedBanner = sinon.stub();
      userHasLinked = sinon.stub();
      dismissHcpLinkBanner = sinon.stub();
    }
    this.owner.register('service:hcp-link-status', HcpLinkStatusStub);
    await render(hbs`<LinkToHcpBanner />`);
    assert.dom(bannerSelector).doesNotExist();
  });

  test('it displays different banner text when consul is enterprise', async function (assert) {
    class EnvStub extends Service {
      isEnterprise = true;
      var(key) {
        return key;
      }
    }
    this.owner.register('service:env', EnvStub);
    await render(hbs`<LinkToHcpBanner />`);
    assert
      .dom('[data-test-link-to-hcp-banner-description]')
      .hasText(
        'By linking your clusters to HCP Consul Central, you’ll get global, cross-cluster metrics, visual service maps, and a global API. HCP Consul Central’s full feature set is included with an Enterprise license.'
      );
  });
});
