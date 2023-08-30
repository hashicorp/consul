/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { render } from '@ember/test-helpers';

module('Integration | Component | consul datacenter selector', function (hooks) {
  setupRenderingTest(hooks);

  test('it does not display a dropdown when only one dc is available', async function (assert) {
    const dcs = [
      {
        Name: 'dc-1',
      },
    ];
    this.set('dcs', dcs);
    this.set('dc', dcs[0]);

    await render(hbs`<Consul::Datacenter::Selector @dcs={{this.dcs}} @dc={{this.dc}} />`);

    assert
      .dom('[data-test-datacenter-disclosure-menu]')
      .doesNotExist('datacenter dropdown is not displayed in nav');

    assert
      .dom('[data-test-datacenter-single]')
      .hasText('dc-1', 'Datecenter name is displayed in nav');
  });

  test('it does displays a dropdown when more than one dc is available', async function (assert) {
    const dcs = [
      {
        Name: 'dc-1',
      },
      {
        Name: 'dc-2',
      },
    ];
    this.set('dcs', dcs);
    this.set('dc', dcs[0]);

    await render(hbs`<Consul::Datacenter::Selector @dcs={{this.dcs}} @dc={{this.dc}} />`);

    assert
      .dom('[data-test-datacenter-single]')
      .doesNotExist('we are displaying more than just the name of the first dc');

    assert.dom('[data-test-datacenter-disclosure-menu]').exists('datacenter dropdown is displayed');
  });
});
