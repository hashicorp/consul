/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import keyToArray from 'consul-ui/utils/keyToArray';

module('Unit | Utils | keyToArray', function () {
  test('it splits a string by a separator, unless the string is the separator', function (assert) {
    assert.expect(4);

    [
      {
        test: '/',
        expected: [''],
      },
      {
        test: 'hello/world',
        expected: ['hello', 'world'],
      },
      {
        test: '/hello/world',
        expected: ['', 'hello', 'world'],
      },
      {
        test: '//',
        expected: ['', '', ''],
      },
    ].forEach(function (item) {
      const actual = keyToArray(item.test);
      assert.deepEqual(actual, item.expected);
    });
  });
});
