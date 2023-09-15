/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/node';

module('Unit | Search | Predicate | node', function () {
  test('items are found by name', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Node: 'node-HIT',
          Address: '10.0.0.0',
        },
        {
          Node: 'node',
          Address: '10.0.0.0',
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 1);
  });
  test('items are found by IP address', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Node: 'node-HIT',
          Address: '10.0.0.0',
        },
      ],
      {
        finders: predicates,
      }
    ).search('10');
    assert.equal(actual.length, 1);
  });
  test('items are not found by name', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Node: 'name',
          Address: '10.0.0.0',
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
  test('items are not found by IP address', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Node: 'name',
          Address: '10.0.0.0',
        },
      ],
      {
        finders: predicates,
      }
    ).search('9');
    assert.equal(actual.length, 0);
  });
});
