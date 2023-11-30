/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | hashicorp consul', function (hooks) {
  setupRenderingTest(hooks);

  skip('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{hashicorp-consul}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    await render(hbs`
      {{#hashicorp-consul}}
      {{/hashicorp-consul}}
    `);

    assert.dom('*').hasText('template block text');
  });
});
