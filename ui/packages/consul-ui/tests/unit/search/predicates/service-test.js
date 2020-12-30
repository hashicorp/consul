import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/service';

module('Unit | Search | Predicate | service', function() {
  test('items are found by properties', function(assert) {
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
  test('items are not found', function(assert) {
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
  test('tags can be empty', function(assert) {
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
});
