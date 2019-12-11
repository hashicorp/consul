import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';
module('Integration | Serializer | acl', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const nspace = 'default';
  const id = 'token-name';
  test('respondForQuery returns the correct data for list endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:acl');
    const request = {
      url: `/v1/acl/list?dc=${dc}`,
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
    const serializer = this.owner.lookup('serializer:acl');
    const request = {
      url: `/v1/acl/info/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = Object.assign({}, payload[0], {
        Datacenter: dc,
        [META]: {},
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
});
