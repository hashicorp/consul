/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import comparators from 'consul-ui/sort/comparators/service';
import { properties } from 'consul-ui/services/sort';
import { module, test } from 'qunit';

module('Unit | Sort | Comparator | service', function () {
  const comparator = comparators({ properties });
  test('Passing anything but Status: just returns what you gave it', function (assert) {
    const expected = 'Name:asc';
    const actual = comparator(expected);
    assert.equal(actual, expected);
  });
  test('items are sorted by a fake Status which uses MeshChecks{Passing,Warning,Critical}', function (assert) {
    const items = [
      {
        MeshChecksPassing: 1,
        MeshChecksWarning: 1,
        MeshChecksCritical: 1,
      },
      {
        MeshChecksPassing: 1,
        MeshChecksWarning: 1,
        MeshChecksCritical: 2,
      },
      {
        MeshChecksPassing: 1,
        MeshChecksWarning: 1,
        MeshChecksCritical: 3,
      },
    ];
    const comp = comparator('Status:asc');
    assert.equal(typeof comp, 'function');

    const expected = [
      {
        MeshChecksPassing: 1,
        MeshChecksWarning: 1,
        MeshChecksCritical: 3,
      },
      {
        MeshChecksPassing: 1,
        MeshChecksWarning: 1,
        MeshChecksCritical: 2,
      },
      {
        MeshChecksPassing: 1,
        MeshChecksWarning: 1,
        MeshChecksCritical: 1,
      },
    ];
    let actual = items.sort(comp);
    assert.deepEqual(actual, expected);

    expected.reverse();
    actual = items.sort(comparator('Status:desc'));
    assert.deepEqual(actual, expected);
  });
});
