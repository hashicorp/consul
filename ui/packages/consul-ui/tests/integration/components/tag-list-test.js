/**
 * Copyright IBM Corp. 2024, 2026
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

    assert.dom('[data-test-tags]').hasText('tag');

    // Template block usage:
    await render(hbs`
      <TagList @item={{hash Tags=(array 'tag')}} as |Tags|>
        <Tags />
      </TagList>
    `);

    assert.dom('[data-test-tags]').hasText('tag');
  });

  test('it renders long tags in full without truncation or overflow clipping', async function (assert) {
    this.set('tags', ['traefik.http.routers.3d-tracking-service.entrypoints=http_default']);
    await render(hbs`<TagList @tags={{this.tags}} />`);

    const card = this.element.querySelector('.tag-list');
    const textContainer = this.element.querySelector('.hds-tag__text-container');
    assert.notStrictEqual(getComputedStyle(card).overflow, 'hidden');
    assert.strictEqual(textContainer.scrollHeight, textContainer.clientHeight);
    assert.dom('.hds-tooltip-button').doesNotExist();
  });
});
