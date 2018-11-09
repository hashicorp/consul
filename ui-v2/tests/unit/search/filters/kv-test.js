import getFilter from 'consul-ui/search/filters/kv';
import { module, test } from 'qunit';

module('Unit | Search | Filter | kv');

const filter = getFilter(cb => cb);
test('items are found by properties', function(assert) {
  [
    {
      Key: 'HIT-here',
    },
    {
      Key: 'folder-HIT/',
    },
    {
      Key: 'really/long/path/HIT-here',
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
      Key: 'key',
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
