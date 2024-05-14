/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { EnvStub } from 'consul-ui/services/env';

module('Integration | Component | hcp nav item', function (hooks) {
  setupRenderingTest(hooks);

  test('it prints the value of CONSUL_HCP_URL', async function (assert) {
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

    assert.dom('[data-test-back-to-hcp]').isVisible();
    assert.dom('a').hasAttribute('href', 'http://hcp.com');
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

    assert.dom('[data-test-back-to-hcp]').doesNotExist();
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

    assert.dom('[data-test-back-to-hcp]').doesNotExist();
    assert.dom('a').doesNotExist();
  });
});
