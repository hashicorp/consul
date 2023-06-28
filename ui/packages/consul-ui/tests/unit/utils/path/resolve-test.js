/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import resolve from 'consul-ui/utils/path/resolve';
import { module, test } from 'qunit';

module('Unit | Utility | path/resolve', function () {
  test('it resolves paths', function (assert) {
    assert.expect(9);
    [
      {
        from: 'dc/intentions/create',
        to: '../edit',
        expected: 'dc/intentions/edit',
      },
      {
        from: 'dc/intentions/create',
        to: '../../edit',
        expected: 'dc/edit',
      },
      {
        from: 'dc/intentions/create',
        to: './edit',
        expected: 'dc/intentions/create/edit',
      },
      {
        from: 'dc/intentions/create',
        to: '././edit',
        expected: 'dc/intentions/create/edit',
      },
      {
        from: 'dc/intentions/create',
        to: './deep/edit',
        expected: 'dc/intentions/create/deep/edit',
      },
      {
        from: 'dc/intentions/create',
        to: '../deep/edit',
        expected: 'dc/intentions/deep/edit',
      },
      {
        from: 'dc/intentions/create',
        to: '.././edit',
        expected: 'dc/intentions/edit',
      },
      {
        from: 'dc/intentions/create',
        to: '../deep/./edit',
        expected: 'dc/intentions/deep/edit',
      },
      {
        from: 'dc/intentions/create',
        to: '/deep/edit',
        expected: '/deep/edit',
      },
    ].forEach((item) => {
      const actual = resolve(item.from, item.to);
      assert.equal(
        actual,
        item.expected,
        `Expected '${item.from}' < '${item.to}' to equal ${item.expected}`
      );
    });
  });
});
