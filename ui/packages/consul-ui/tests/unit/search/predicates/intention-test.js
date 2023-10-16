/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/intention';

module('Unit | Search | Predicate | intention', function () {
  test('items are found by properties', function (assert) {
    const actual = new ExactSearch(
      [
        {
          SourceName: 'Hit',
          DestinationName: 'destination',
        },
        {
          SourceName: 'source',
          DestinationName: 'destination',
        },
        {
          SourceName: 'source',
          DestinationName: 'hiT',
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
          SourceName: 'source',
          DestinationName: 'destination',
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
  test('items are found by *', function (assert) {
    const actual = new ExactSearch(
      [
        {
          SourceName: '*',
          DestinationName: 'destination',
        },
        {
          SourceName: 'source',
          DestinationName: '*',
        },
      ],
      {
        finders: predicates,
      }
    ).search('*');
    assert.equal(actual.length, 2);
  });
  test("* items are found by searching anything in 'All Services (*)'", function (assert) {
    assert.expect(6);

    const actual = new ExactSearch(
      [
        {
          SourceName: '*',
          DestinationName: 'destination',
        },
        {
          SourceName: 'source',
          DestinationName: '*',
        },
      ],
      {
        finders: predicates,
      }
    );
    ['All Services (*)', 'SerVices', '(*)', '*', 'vIces', 'lL Ser'].forEach((term) => {
      assert.equal(actual.search(term).length, 2);
    });
  });
});
