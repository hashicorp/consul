/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | state-matches', function (hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it returns true/false when the state or state in an array matches', async function (assert) {
    this.set('state', {
      matches: function (id) {
        return id === 'idle';
      },
    });

    await render(hbs`{{state-matches state 'idle'}}`);
    assert.equal(this.element.textContent.trim(), 'true');

    await render(hbs`{{state-matches state 'loading'}}`);
    assert.equal(this.element.textContent.trim(), 'false');

    await render(hbs`{{state-matches state (array 'idle' 'loading')}}`);
    assert.equal(this.element.textContent.trim(), 'true');

    await render(hbs`{{state-matches state (array 'loading' 'idle')}}`);
    assert.equal(this.element.textContent.trim(), 'true');

    await render(hbs`{{state-matches state (array 'loading' 'deleting')}}`);
    assert.equal(this.element.textContent.trim(), 'false');
  });
});
