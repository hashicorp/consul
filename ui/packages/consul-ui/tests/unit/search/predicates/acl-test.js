import predicates from 'consul-ui/search/predicates/acl';
import { search as create } from 'consul-ui/services/search';
import { module, test } from 'qunit';

module('Unit | Search | Predicate | acl', function() {
  const search = create(predicates);
  test('items are found by properties', function(assert) {
    const actual = [
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
    ].filter(search('hit'));
    assert.equal(actual.length, 2);
  });
  test('items are not found', function(assert) {
    const actual = [
      {
        ID: 'id',
        Name: 'name',
      },
    ].filter(search('hit'));
    assert.equal(actual.length, 0);
  });
});
