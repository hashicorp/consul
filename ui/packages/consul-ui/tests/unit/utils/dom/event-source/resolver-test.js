/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import domEventSourceResolver from 'consul-ui/utils/dom/event-source/resolver';
import { module, test } from 'qunit';

module('Unit | Utility | dom/event source/resolver', function () {
  // Replace this with your real tests.
  test('it works', function (assert) {
    let result = domEventSourceResolver();
    assert.ok(result);
  });
});
