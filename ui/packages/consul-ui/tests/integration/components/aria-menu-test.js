/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | aria-menu', function (hooks) {
  setupRenderingTest(hooks);

  skip('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`{{aria-menu}}`);

    assert.equal(this.element.textContent.trim(), '');

    // Template block usage:
    await render(hbs`
      {{#aria-menu}}
        template block text
      {{/aria-menu}}
    `);

    assert.equal(this.element.textContent.trim(), 'template block text');
  });
  skip('it responds to keyboard events');
});
