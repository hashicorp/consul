/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import updateArrayObject from 'consul-ui/utils/update-array-object';
import { module, test } from 'qunit';

module('Unit | Utility | update array object', function () {
  // Replace this with your real tests.
  test('it updates the correct item in the array', function (assert) {
    const expected = {
      data: {
        id: '2',
        name: 'expected',
      },
    };
    const arr = [
      {
        data: {
          id: '1',
          name: 'name',
        },
      },
      {
        data: {
          id: '2',
          name: '-',
        },
      },
    ];
    const actual = updateArrayObject(arr, expected, 'id');
    assert.ok(actual, expected);
    assert.equal(arr[1].name, expected.name);
  });
});
