/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import callIfType from 'consul-ui/utils/helpers/call-if-type';
import { module, test } from 'qunit';

module('Unit | Utility | helpers/call if type', function () {
  test('it calls the function if the correct helper argument is passed', function (assert) {
    const helper = callIfType('number')(function () {
      return true;
    });
    assert.ok(helper([1]));
  });
  test('it returns the same argument if the incorrect helper argument is passed', function (assert) {
    const helper = callIfType('number')(function () {
      return true;
    });
    const expected = 'hi';
    const actual = helper(['hi']);

    assert.equal(actual, expected);
  });
});
