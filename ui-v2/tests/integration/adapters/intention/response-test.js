import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
module('Integration | Adapter | intention | response', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'intention-name';
  test('handleResponse returns the correct data for list endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const request = {
      url: `/v1/connect/intentions?dc=${dc}`,
      method: 'GET',
    };
    return get(request.url).then(function(payload) {
      const expected = payload.map(item =>
        Object.assign({}, item, {
          Datacenter: dc,
          uid: `["${dc}","${item.ID}"]`,
        })
      );
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
  test('handleResponse returns the correct data for item endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const request = {
      url: `/v1/connect/intentions/${id}?dc=${dc}`,
      method: 'GET',
    };
    return get(request.url).then(function(payload) {
      const expected = Object.assign({}, payload, {
        Datacenter: dc,
        uid: `["${dc}","${id}"]`,
      });
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
});
