import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
module('Integration | Adapter | dc | response', function(hooks) {
  setupTest(hooks);
  test('handleResponse returns the correct data for list endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:dc');
    const request = {
      url: `/v1/catalog/datacenters`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload;
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
});
