import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/acl';

module('Unit | Search | Predicate | acl', function() {
  test('items are found by properties', function(assert) {
    const actual = new ExactSearch(
      [
        {
          ID: 'HIT-id',
          Name: 'name',
        },
        {
          ID: 'id',
          Name: 'name',
        },
        {
          ID: 'id',
          Name: 'name-HIT',
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
          ID: 'id',
          Name: 'name',
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
});
