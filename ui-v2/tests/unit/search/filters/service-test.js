import getFilter from 'consul-ui/search/filters/service';
import { module, test } from 'qunit';

module('Unit | Search | Filter | service');

const filter = getFilter(cb => cb);
test('items are found by properties', function(assert) {
  [
    {
      Name: 'name-HIT',
      Tags: [],
    },
    {
      Name: 'name',
      Tags: ['tag', 'tag-withHiT'],
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
    },
    {
      Name: 'name',
      Tags: ['one', 'two'],
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
test('tags can be empty', function(assert) {
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
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
