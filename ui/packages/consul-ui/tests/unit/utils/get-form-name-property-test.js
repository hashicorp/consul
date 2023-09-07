/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import getFormNameProperty from 'consul-ui/utils/get-form-name-property';
import { module, test } from 'qunit';

module('Unit | Utility | get form name property', function () {
  // Replace this with your real tests.
  test("it parses 'item[property]' to `['item',' property']`", function (assert) {
    const expected = ['item', 'property'];
    const actual = getFormNameProperty(`${expected[0]}[${expected[1]}]`);
    assert.deepEqual(actual, expected);
  });
});
