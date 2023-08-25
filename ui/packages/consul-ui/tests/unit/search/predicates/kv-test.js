/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/kv';

module('Unit | Search | Predicate | kv', function () {
  test('items are found by properties', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Key: 'HIT-here',
        },
        {
          Key: 'folder-HIT/',
        },
        {
          Key: 'excluded',
        },
        {
          Key: 'really/long/path/HIT-here',
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 3);
  });
  test('items are not found', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Key: 'key',
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
});
