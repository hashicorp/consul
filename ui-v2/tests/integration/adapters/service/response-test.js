import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
module('Integration | Adapter | service | response', function(hooks) {
  setupTest(hooks);
  test('handleResponse returns the correct data for list endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:service');
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
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
  test('handleResponse returns the correct data for item endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:service');
    const dc = 'dc-1';
    const id = 'service-name';
    const request = {
      url: `/v1/health/service/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = {
        Datacenter: dc,
        uid: `["${dc}","${id}"]`,
        Nodes: payload,
      };
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
});
