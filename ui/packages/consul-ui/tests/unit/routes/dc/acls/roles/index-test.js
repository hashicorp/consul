/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | dc/acls/roles/index', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:dc/acls/roles/index');
    assert.ok(route);
  });
});
