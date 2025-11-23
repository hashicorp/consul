/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Modifier | click-outside', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    const modifier = this.owner.lookup('modifier:click-outside');
    assert.ok(modifier, 'click-outside modifier exists');
  });

  test('it has required methods', function (assert) {
    const modifier = this.owner.lookup('modifier:click-outside');
    assert.strictEqual(typeof modifier.modify, 'function', 'has modify method');
    assert.strictEqual(typeof modifier.willDestroy, 'function', 'has willDestroy method');
  });
});
