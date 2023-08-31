/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | tabular collection', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{tabular-collection cell-layout=(fixed-grid-layout 800 50)}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    await render(hbs`
      {{#tabular-collection cell-layout=(fixed-grid-layout 800 50)}}{{/tabular-collection}}
    `);

    assert.dom('*').hasText('');
  });
});
