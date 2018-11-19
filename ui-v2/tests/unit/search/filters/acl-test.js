import getFilter from 'consul-ui/search/filters/acl';
import { module, test } from 'qunit';

module('Unit | Search | Filter | acl');

const filter = getFilter(cb => cb);
test('items are found by properties', function(assert) {
  [
    {
      ID: 'HIT-id',
      Name: 'name',
    },
    {
      ID: 'id',
      Name: 'name-HIT',
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
      ID: 'id',
      Name: 'name',
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
