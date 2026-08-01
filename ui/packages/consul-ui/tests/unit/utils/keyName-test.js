/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import keyName from 'consul-ui/utils/keyName';

module('Unit | Utils | keyName', function () {
  test('it returns the last non-empty segment of a key', function (assert) {
    [
      {
        test: 'hello/world',
        expected: 'world',
      },
      {
        test: 'hello/world/',
        expected: 'world',
      },
      {
        test: '/hello',
        expected: 'hello',
      },
      {
        test: 'hello',
        expected: 'hello',
      },
      {
        test: '/',
        expected: undefined,
      },
      {
        test: undefined,
        expected: undefined,
      },
    ].forEach(function (item) {
      const actual = keyName(item.test);
      assert.strictEqual(actual, item.expected);
    });
  });
});
