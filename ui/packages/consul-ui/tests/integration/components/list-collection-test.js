/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, triggerEvent } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | list collection', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{list-collection cell-layout=(fixed-grid-layout 800 50)}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    await render(hbs`
      {{#list-collection cell-layout=(fixed-grid-layout 800 50)}}{{/list-collection}}
    `);

    assert.dom('*').hasText('');
  });

  test('it manages checked state and z-index on change', async function (assert) {
    this.set('items', [
      { id: 1, name: 'Item 1' },
      { id: 2, name: 'Item 2' },
    ]);

    // Add a footer for collision detection
    const footer = document.createElement('footer');
    footer.setAttribute('role', 'contentinfo');
    footer.style.position = 'fixed';
    footer.style.bottom = '0';
    footer.style.height = '50px';
    document.body.appendChild(footer);

    await render(hbs`
      <ListCollection @items={{this.items}} as |item index Actions|>
        <BlockSlot @name="header">{{item.name}}</BlockSlot>
        <BlockSlot @name="actions" as |Actions|>
          <Actions as |Action|>
            <Action>
              <BlockSlot @name="label">Action</BlockSlot>
            </Action>
          </Actions>
        </BlockSlot>
      </ListCollection>
    `);

    const checkbox = this.element.querySelector('input[type="checkbox"]');
    const row = this.element.querySelector('[data-test-list-row]');

    // Test checking - should set z-index and handle footer collision
    checkbox.checked = true;
    await triggerEvent(checkbox, 'change');
    assert.equal(row.style.zIndex, '1', 'Row should have z-index 1 when checked');

    // Test unchecking - should clear z-index
    checkbox.checked = false;
    await triggerEvent(checkbox, 'change');
    assert.strictEqual(row.style.zIndex, '', 'Row z-index should be cleared when unchecked');

    // Cleanup
    document.body.removeChild(footer);
  });
});
