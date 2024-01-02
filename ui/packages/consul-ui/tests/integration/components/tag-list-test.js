/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | tag list', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`<TagList @item={{hash Tags=(array 'tag')}} />`);

    assert.dom('dd').hasText('tag');

    // Template block usage:
    await render(hbs`
      <TagList @item={{hash Tags=(array 'tag')}} as |Tags|>
        <Tags />
      </TagList>
    `);

    assert.dom('dd').hasText('tag');
  });
});
