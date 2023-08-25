/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Service | token', function (hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function (assert) {
    let service = this.owner.lookup('service:repository/token');
    assert.ok(service);
  });
});
