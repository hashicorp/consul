import getFilter from 'consul-ui/search/filters/service/node';
import { module, test } from 'qunit';

module('Unit | Search | Filter | service/node');

const filter = getFilter(cb => cb);
test('items are found by properties', function(assert) {
  [
    {
      Service: {
        ID: 'hit',
      },
      Node: {
        Node: 'node',
      },
    },
    {
      Service: {
        ID: 'id',
      },
      Node: {
        Node: 'nodeHiT',
      },
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.ok(actual);
  });
});
test('items are found by address:port', function(assert) {
  const instance = {
    Service: {
      ID: 'id',
      Address: '0.0.0.0',
      Port: 8000,
    },
    Node: {
      Node: 'node-0',
    },
  };
  ['0.0.0.0', '8000', '0:8000', '0.0.0.0:8000'].forEach(function(item) {
    let actual = filter(instance, {
      s: item,
    });
    assert.ok(actual);
  });
});
test('items are not found', function(assert) {
  [
    {
      Service: {
        ID: 'ID',
      },
      Node: {
        Node: 'node',
      },
    },
    {
      Service: {
        ID: 'id',
      },
      Node: {
        Node: 'node',
      },
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
