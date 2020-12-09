import predicates from 'consul-ui/search/predicates/policy';
import { search as create } from 'consul-ui/services/search';
import { module, test } from 'qunit';

module('Unit | Search | Predicate | policy', function() {
  const search = create(predicates);
  test('items are found by properties', function(assert) {
    const actual = [
      {
        Name: 'name-HIT',
        Description: 'description',
      },
      {
        Name: 'name',
        Description: 'desc-HIT-ription',
      },
    ].filter(search('hit'));
    assert.equal(actual.length, 2);
  });
  test('items are not found', function(assert) {
    const actual = [
      {
        Name: 'name',
        Description: 'description',
      },
    ].filter(search('hit'));
    assert.equal(actual.length, 0);
  });
});
