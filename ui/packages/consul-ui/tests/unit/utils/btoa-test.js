/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import btoa from 'consul-ui/utils/btoa';

module('Unit | Utils | btoa', function () {
  test('it encodes strings properly', function (assert) {
    [
      {
        test: '',
        expected: '',
      },
      {
        test: '1234',
        expected: 'MTIzNA==',
      },
    ].forEach(function (item) {
      const actual = btoa(item.test);
      assert.strictEqual(actual, item.expected);
    });
  });
});
