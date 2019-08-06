import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';
module('Integration | Serializer | service', function(hooks) {
  setupTest(hooks);
  test('respondForQuery returns the correct data for list endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:service');
    const dc = 'dc-1';
    const request = {
      url: `/v1/internal/ui/services?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload.map(item =>
        Object.assign({}, item, {
          Datacenter: dc,
          uid: `["${dc}","${item.Name}"]`,
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
    const serializer = this.owner.lookup('serializer:service');
    const dc = 'dc-1';
    const id = 'service-name';
    const request = {
      url: `/v1/health/service/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = {
        Datacenter: dc,
        [META]: {},
        uid: `["${dc}","${id}"]`,
        Name: id,
        Nodes: payload,
      };
      const actual = serializer.respondForQueryRecord(
        function(cb) {
          const headers = {};
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
          id: id,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
});
