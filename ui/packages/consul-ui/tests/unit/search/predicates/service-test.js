/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/service';

module('Unit | Search | Predicate | service', function () {
  test('items are found by properties', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Name: 'name-HIT',
          Tags: [],
        },
        {
          Name: 'name',
          Tags: ['tag', 'tag-withHiT'],
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 2);
  });
  test('items are not found', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Name: 'name',
        },
        {
          Name: 'name',
          Tags: ['one', 'two'],
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
  test('tags can be empty', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Name: 'name',
        },
        {
          Name: 'name',
          Tags: null,
        },
        {
          Name: 'name',
          Tags: [],
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
  test('items can be found by Partition', function (assert) {
    const search = new ExactSearch(
      [
        {
          Name: 'name-a',
          Partition: 'default',
        },
        {
          Name: 'name-b',
          Partition: 'lorem-ipsum',
        },
      ],
      {
        finders: predicates,
      }
    );

    assert.deepEqual(
      search.search('').map((i) => i.Name),
      ['name-a', 'name-b'],
      'both items included in search'
    );

    assert.deepEqual(
      search.search('def').map((i) => i.Name),
      ['name-a'],
      'only item from default partition is included'
    );

    assert.deepEqual(
      search.search('tomster').map((i) => i.Name),
      [],
      'no item included when no Partition matches'
    );
  });
});
