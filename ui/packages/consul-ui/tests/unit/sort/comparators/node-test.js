/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import comparators from 'consul-ui/sort/comparators/node';
import { properties } from 'consul-ui/services/sort';
import { module, test } from 'qunit';

module('Unit | Sort | Comparator | node', function () {
  const comparator = comparators({ properties });
  test('items are sorted by a fake Version', function (assert) {
    const items = [
      {
        Version: '2.24.1',
      },
      {
        Version: '1.12.6',
      },
      {
        Version: '2.09.3',
      },
    ];
    const comp = comparator('Version:asc');
    assert.equal(typeof comp, 'function');

    const expected = [
      {
        Version: '1.12.6',
      },
      {
        Version: '2.09.3',
      },
      {
        Version: '2.24.1',
      },
    ];
    let actual = items.sort(comp);
    assert.deepEqual(actual, expected);

    expected.reverse();
    actual = items.sort(comparator('Version:desc'));
    assert.deepEqual(actual, expected);
  });
});
