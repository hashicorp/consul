/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

module('Integration | Modifier | fix-super-select-aria', function (hooks) {
  setupRenderingTest(hooks);

  test('it changes role="alert" to role="option"', async function (assert) {
    await render(hbs`
      <div {{fix-super-select-aria}}>
        <span role="alert" aria-selected="true"></span>
      </div>
    `);
    await wait(150); // Wait longer than the 100ms timeout
    assert.dom('[role="option"][aria-selected]').exists('role changed to option');
  });

  test('it removes invalid aria-controls and adds aria-expanded', async function (assert) {
    await render(hbs`
      <div {{fix-super-select-aria}}>
        <span role="combobox" aria-controls="missing"></span>
      </div>
    `);
    await wait(150);
    assert.dom('[role="combobox"]').doesNotHaveAttribute('aria-controls', 'aria-controls removed');
    assert.dom('[role="combobox"]').hasAttribute('aria-expanded', 'false', 'aria-expanded added');
  });

  test('it adds missing aria-label to listboxes', async function (assert) {
    await render(hbs`
      <div {{fix-super-select-aria}}>
        <div role="listbox"></div>
      </div>
    `);
    await wait(150);
    assert
      .dom('[role="listbox"]')
      .hasAttribute('aria-label', 'Available Options', 'aria-label added to listbox');
  });
});
