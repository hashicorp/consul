import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
} from 'consul-ui/utils/http/consul';
module('Integration | Serializer | node', function(hooks) {
  setupTest(hooks);
  const nspace = 'default';
  test('respondForQuery returns the correct data for list endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:node');
    const dc = 'dc-1';
    const request = {
      url: `/v1/internal/ui/nodes?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload.map(item =>
        Object.assign({}, item, {
          Datacenter: dc,
          // TODO: default isn't required here, once we've
          // refactored out our Serializer this can go
          Namespace: nspace,
          uid: `["${nspace}","${dc}","${item.ID}"]`,
        })
      );
      const actual = serializer.respondForQuery(
        function(cb) {
          const headers = {};
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
  test('respondForQueryRecord returns the correct data for item endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:node');
    const dc = 'dc-1';
    const id = 'node-name';
    const request = {
      url: `/v1/internal/ui/node/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = Object.assign({}, payload, {
        Datacenter: dc,
        [META]: {
          [DC.toLowerCase()]: dc,
          [NSPACE.toLowerCase()]: nspace,
        },
        // TODO: default isn't required here, once we've
        // refactored out our Serializer this can go
        Namespace: nspace,
        uid: `["${nspace}","${dc}","${id}"]`,
      });
      const actual = serializer.respondForQueryRecord(
        function(cb) {
          const headers = {};
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
  test('respondForQueryLeader returns the correct data', function(assert) {
    const serializer = this.owner.lookup('serializer:node');
    const dc = 'dc-1';
    const request = {
      url: `/v1/status/leader?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = {
        Address: '211.245.86.75',
        Port: '8500',
      };
      const actual = serializer.respondForQueryLeader(
        function(cb) {
          const headers = {};
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
});
