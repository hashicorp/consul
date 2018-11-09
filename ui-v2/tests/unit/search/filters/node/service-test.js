import getFilter from 'consul-ui/search/filters/node/service';
import { module, test } from 'qunit';

module('Unit | Search | Filter | node/service');

const filter = getFilter(cb => cb);
test('items are found by properties', function(assert) {
  [
    {
      Service: 'service-HIT',
      ID: 'id',
      Port: 8500,
      Tags: [],
    },
    {
      Service: 'service',
      ID: 'id-HiT',
      Port: 8500,
      Tags: [],
    },
    {
      Service: 'service',
      ID: 'id',
      Port: 8500,
      Tags: ['tag', 'tag-withHiT'],
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.ok(actual);
  });
});
test('items are found by port (non-string)', function(assert) {
  [
    {
      Service: 'service',
      ID: 'id',
      Port: 8500,
      Tags: ['tag', 'tag'],
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: '8500',
    });
    assert.ok(actual);
  });
});
test('items are not found', function(assert) {
  [
    {
      Service: 'service',
      ID: 'id',
      Port: 8500,
    },
    {
      Service: 'service',
      ID: 'id',
      Port: 8500,
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
      Service: 'service',
      ID: 'id',
      Port: 8500,
    },
    {
      Service: 'service',
      ID: 'id',
      Port: 8500,
      Tags: null,
    },
    {
      Service: 'service',
      ID: 'id',
      Port: 8500,
      Tags: [],
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
