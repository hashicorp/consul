/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Modifier | element-ref', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    const modifier = this.owner.lookup('modifier:element-ref');
    assert.ok(modifier, 'element-ref modifier exists');
  });
});
