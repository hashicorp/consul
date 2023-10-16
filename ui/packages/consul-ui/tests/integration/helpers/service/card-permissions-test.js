/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | service/card-permissions', function (hooks) {
  setupRenderingTest(hooks);

  // TODO: Replace this with your real tests.
  test('it renders', async function (assert) {
    this.set('inputValue', {
      Intention: {
        Allowed: false,
        HasPermissions: true,
      },
    });

    await render(hbs`{{service/card-permissions inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'allow');
  });
});
