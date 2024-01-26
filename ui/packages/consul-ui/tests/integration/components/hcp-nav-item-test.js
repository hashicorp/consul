/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { EnvStub } from 'consul-ui/services/env';
import Service from '@ember/service';

const backToHcpSelector = '[data-test-back-to-hcp]';
const hcpConsulCentralItemSelector = '[data-test-linked-cluster-hcp-link]';
const linkToHcpSelector = '[data-test-link-to-hcp]';
const linkToHcpNewBadgeSelector = '[data-test-link-to-hcp-new-badge]';
const resourceId =
  'organization/b4432207-bb9c-438e-a160-b98923efa979/project/4b09958c-fa91-43ab-8029-eb28d8cee9d4/hashicorp.consul.global-network-manager.cluster/test-from-api';
module('Integration | Component | hcp nav item', function (hooks) {
  setupRenderingTest(hooks);

  module('back to hcp item', function () {
    test('it prints the value of CONSUL_HCP_URL when env vars are set', async function (assert) {
      this.owner.register(
        'service:env',
        class Stub extends EnvStub {
          stubEnv = {
            CONSUL_HCP_URL: 'http://hcp.com',
            CONSUL_HCP_ENABLED: true,
          };
        }
      );

      await render(hbs`
      <Hds::SideNav::List as |SNL|>
        <HcpNavItem @list={{SNL}} />
      </Hds::SideNav::List>
    `);

      assert.dom(backToHcpSelector).isVisible();
      assert.dom('a').hasAttribute('href', 'http://hcp.com');
      assert.dom(linkToHcpSelector).doesNotExist('link to hcp should not be visible');
      assert
        .dom(hcpConsulCentralItemSelector)
        .doesNotExist('hcp consul central item should not be visible');
    });

    test('it does not output the Back to HCP link if CONSUL_HCP_URL is not present', async function (assert) {
      this.owner.register(
        'service:env',
        class Stub extends EnvStub {
          stubEnv = {
            CONSUL_HCP_ENABLED: true,
            CONSUL_HCP_URL: undefined,
          };
        }
      );

      await render(hbs`
      <Hds::SideNav::List as |SNL|>
        <HcpNavItem @list={{SNL}} />
      </Hds::SideNav::List>
    `);

      assert.dom(backToHcpSelector).doesNotExist();
      assert.dom('a').doesNotExist();
    });
    test('it does not output the Back to HCP link if CONSUL_HCP_ENABLED is not present', async function (assert) {
      this.owner.register(
        'service:env',
        class Stub extends EnvStub {
          stubEnv = {
            CONSUL_HCP_URL: 'http://hcp.com',
            CONSUL_HCP_ENABLED: undefined,
          };
        }
      );

      await render(hbs`
      <Hds::SideNav::List as |SNL|>
        <HcpNavItem @list={{SNL}} />
      </Hds::SideNav::List>
    `);

      assert.dom(backToHcpSelector).doesNotExist();
      assert.dom('a').doesNotExist();
    });
  });

  module('when rendered in self managed mode', function (hooks) {
    hooks.beforeEach(function () {
      this.owner.register(
        'service:env',
        class Stub extends EnvStub {
          stubEnv = {};
        }
      );
    });

    test('when unauthorized to link it does not display any nav items', async function (assert) {
      this.owner.register(
        'service:hcp-link-status',
        class Stub extends Service {
          hasPermissionToLink = false;
        }
      );
      this.linkData = {
        resourceId,
        isLinked: false,
      };
      await render(hbs`
        <Hds::SideNav::List as |SNL|>
          <HcpNavItem @list={{SNL}} @linkData={{this.linkData}}/>
        </Hds::SideNav::List>
      `);
      assert.dom(backToHcpSelector).doesNotExist('back to hcp should not be visible');
      assert.dom(linkToHcpSelector).doesNotExist('link to hcp should not be visible');
      assert
        .dom(hcpConsulCentralItemSelector)
        .doesNotExist('hcp consul central item should not be visible');
    });

    test('when link status is undefined it does not display any nav items', async function (assert) {
      this.owner.register(
        'service:hcp-link-status',
        class Stub extends Service {
          hasPermissionToLink = true;
        }
      );
      this.linkData = {
        resourceId,
      };
      await render(hbs`
        <Hds::SideNav::List as |SNL|>
          <HcpNavItem @list={{SNL}} @linkData={{this.linkData}}/>
        </Hds::SideNav::List>
      `);
      assert.dom(backToHcpSelector).doesNotExist('back to hcp should not be visible');
      assert.dom(linkToHcpSelector).doesNotExist('link to hcp should not be visible');
      assert
        .dom(hcpConsulCentralItemSelector)
        .doesNotExist('hcp consul central item should not be visible');
    });

    test('when already linked but no resourceId it does not display any nav items', async function (assert) {
      this.owner.register(
        'service:hcp-link-status',
        class Stub extends Service {
          hasPermissionToLink = true;
        }
      );
      this.linkData = {
        isLinked: true,
      };
      await render(hbs`
        <Hds::SideNav::List as |SNL|>
          <HcpNavItem @list={{SNL}} @linkData={{this.linkData}}/>
        </Hds::SideNav::List>
      `);
      assert.dom(backToHcpSelector).doesNotExist('back to hcp should not be visible');
      assert.dom(linkToHcpSelector).doesNotExist('link to hcp should not be visible');
      assert
        .dom(hcpConsulCentralItemSelector)
        .doesNotExist('hcp consul central item should not be visible');
    });

    test('when already linked and we have a resourceId it displays the link to hcp consul central item', async function (assert) {
      this.owner.register(
        'service:hcp-link-status',
        class Stub extends Service {
          hasPermissionToLink = true;
        }
      );
      this.linkData = {
        isLinked: true,
        resourceId,
      };
      await render(hbs`
        <Hds::SideNav::List as |SNL|>
          <HcpNavItem @list={{SNL}} @linkData={{this.linkData}}/>
        </Hds::SideNav::List>
      `);
      assert.dom(backToHcpSelector).doesNotExist('back to hcp should not be visible');
      assert.dom(linkToHcpSelector).doesNotExist('link to hcp should not be visible');
      assert
        .dom(hcpConsulCentralItemSelector)
        .isVisible('hcp consul central item should be visible');
    });

    test('when not already linked without dismissed banner it displays the link to hcp item', async function (assert) {
      this.owner.register(
        'service:hcp-link-status',
        class Stub extends Service {
          hasPermissionToLink = true;
          shouldDisplayBanner = true;
        }
      );
      this.linkData = {
        isLinked: false,
      };
      await render(hbs`
        <Hds::SideNav::List as |SNL|>
          <HcpNavItem @list={{SNL}} @linkData={{this.linkData}}/>
        </Hds::SideNav::List>
      `);
      assert.dom(backToHcpSelector).doesNotExist('back to hcp should not be visible');
      assert
        .dom(hcpConsulCentralItemSelector)
        .doesNotExist('hcp consul central item should not be visible');
      assert.dom(linkToHcpSelector).isVisible('link to hcp should be visible');
      assert.dom(linkToHcpNewBadgeSelector).isVisible('badge should be visible');
    });

    test('when not already linked with dismissed banner it displays the link to hcp item', async function (assert) {
      this.owner.register(
        'service:hcp-link-status',
        class Stub extends Service {
          hasPermissionToLink = true;
          shouldDisplayBanner = false;
        }
      );
      this.linkData = {
        isLinked: false,
      };
      await render(hbs`
        <Hds::SideNav::List as |SNL|>
          <HcpNavItem @list={{SNL}} @linkData={{this.linkData}}/>
        </Hds::SideNav::List>
      `);
      assert.dom(backToHcpSelector).doesNotExist('back to hcp should not be visible');
      assert
        .dom(hcpConsulCentralItemSelector)
        .doesNotExist('hcp consul central item should not be visible');
      assert.dom(linkToHcpSelector).isVisible('link to hcp should be visible');
      assert.dom(linkToHcpNewBadgeSelector).doesNotExist('badge should be visible');
    });
  });
});
