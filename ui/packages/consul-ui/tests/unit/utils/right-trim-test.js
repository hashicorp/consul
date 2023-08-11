/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import rightTrim from 'consul-ui/utils/right-trim';

module('Unit | Utility | right trim', function () {
  test('it trims characters from the right hand side', function (assert) {
    assert.expect(12);

    [
      {
        args: ['/a/folder/here/', '/'],
        expected: '/a/folder/here',
      },
      {
        args: ['/a/folder/here', ''],
        expected: '/a/folder/here',
      },
      {
        args: ['a/folder/here', '/'],
        expected: 'a/folder/here',
      },
      {
        args: ['a/folder/here/', '/'],
        expected: 'a/folder/here',
      },
      {
        args: [],
        expected: '',
      },
      {
        args: ['/a/folder/here', '/folder/here'],
        expected: '/a',
      },
      {
        args: ['/a/folder/here', 'a/folder/here'],
        expected: '/',
      },
      {
        args: ['/a/folder/here/', '/a/folder/here/'],
        expected: '',
      },
      {
        args: ['/a/folder/here/', '-'],
        expected: '/a/folder/here/',
      },
      {
        args: ['/a/folder/here/', 'here'],
        expected: '/a/folder/here/',
      },
      {
        args: ['here', '/here'],
        expected: 'here',
      },
      {
        args: ['/here', '/here'],
        expected: '',
      },
    ].forEach(function (item) {
      const actual = rightTrim(...item.args);
      assert.equal(actual, item.expected);
    });
  });
});
