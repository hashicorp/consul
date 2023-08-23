import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/policy';

module('Unit | Search | Predicate | policy', function () {
  test('items are found by properties', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Name: 'name-HIT',
          Description: 'description',
        },
        {
          Name: 'name',
          Description: 'desc-HIT-ription',
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
          Description: 'description',
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
});
