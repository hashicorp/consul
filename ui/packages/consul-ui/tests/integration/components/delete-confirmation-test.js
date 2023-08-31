/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | delete confirmation', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{delete-confirmation}}`);

    assert.dom('[data-test-delete]').exists({ count: 1 });

    // Template block usage:
    await render(hbs`
      {{#delete-confirmation}}{{/delete-confirmation}}
    `);

    assert.dom('[data-test-delete]').exists({ count: 1 });
  });
});
