import predicates from 'consul-ui/search/predicates/kv';
import { search as create } from 'consul-ui/services/search';
import { module, test } from 'qunit';

module('Unit | Search | Predicate | kv', function() {
  const search = create(predicates);
  test('items are found by properties', function(assert) {
    const actual = [
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
    ].filter(search('hit'));
    assert.equal(actual.length, 3);
  });
  test('items are not found', function(assert) {
    const actual = [
      {
        Key: 'key',
      },
    ].filter(search('hit'));
    assert.equal(actual.length, 0);
  });
});
