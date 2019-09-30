import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';
module('Integration | Serializer | kv', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'key-name/here';
  test('respondForQuery returns the correct data for list endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:kv');
    const request = {
      url: `/v1/kv/${id}?keys&dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload.map(item =>
        Object.assign(
          {},
          {
            Key: item,
          },
          {
            Datacenter: dc,
            uid: `["${dc}","${item}"]`,
          }
        )
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
    const serializer = this.owner.lookup('serializer:kv');
    const request = {
      url: `/v1/kv/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = Object.assign({}, payload[0], {
        Datacenter: dc,
        [META]: {},
        uid: `["${dc}","${id}"]`,
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
});
