import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
module('Integration | Adapter | kv | response', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'key-name/here';
  test('handleResponse returns the correct data for list endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
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
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
  test('handleResponse returns the correct data for item endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const request = {
      url: `/v1/kv/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = Object.assign({}, payload[0], {
        Datacenter: dc,
        uid: `["${dc}","${id}"]`,
      });
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
});
