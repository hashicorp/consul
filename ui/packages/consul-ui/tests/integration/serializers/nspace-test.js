import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { get } from 'consul-ui/tests/helpers/api';
// Nspace don't need any datacenter or nspace, and don't
module('Integration | Serializer | nspace', function(hooks) {
  setupTest(hooks);
  test('respondForQuery returns the correct data for list endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:nspace');
    const request = {
      url: `/v1/namespaces`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload.map(item => Object.assign({}, item, {}));
      const actual = serializer.respondForQuery(function(cb) {
        const headers = {};
        const body = payload;
        return cb(headers, body);
      }, {});
      assert.deepEqual(actual, expected);
    });
  });
  test('respondForQueryRecord returns the correct data for item endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:nspace');
    const id = 'slug';
    const request = {
      url: `/v1/namespace/${id}`,
    };
    return get(request.url).then(function(payload) {
      // Namespace items don't currently get META attached
      const expected = payload;
      const actual = serializer.respondForQueryRecord(
        function(cb) {
          const headers = {};
          const body = payload;
          return cb(headers, body);
        },
        {
          id: id,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
});
