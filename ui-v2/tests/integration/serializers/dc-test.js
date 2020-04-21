import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
module('Integration | Serializer | dc', function(hooks) {
  setupTest(hooks);
  test('respondForQuery returns the correct data for list endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:dc');
    const request = {
      url: `/v1/catalog/datacenters`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload;
      const actual = serializer.respondForQuery(function(cb) {
        const headers = {};
        const body = payload;
        return cb(headers, body);
      });
      assert.deepEqual(actual, expected);
    });
  });
});
