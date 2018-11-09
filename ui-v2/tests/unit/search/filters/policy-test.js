import getFilter from 'consul-ui/search/filters/policy';
import { module, test } from 'qunit';

module('Unit | Search | Filter | policy');

const filter = getFilter(cb => cb);
test('items are found by properties', function(assert) {
  [
    {
      Name: 'name-HIT',
      Description: 'description',
    },
    {
      Name: 'name',
      Description: 'desc-HIT-ription',
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.ok(actual);
  });
});
test('items are not found', function(assert) {
  [
    {
      Name: 'name',
      Description: 'description',
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
