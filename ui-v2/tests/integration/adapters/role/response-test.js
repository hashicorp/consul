import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';
import { createPolicies } from 'consul-ui/tests/helpers/normalizers';

module('Integration | Adapter | role | response', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'role-name';
  test('handleResponse returns the correct data for list endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    const request = {
      url: `/v1/acl/roles?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload.map(item =>
        Object.assign({}, item, {
          Datacenter: dc,
          uid: `["${dc}","${item.ID}"]`,
          Policies: createPolicies(item),
        })
      );
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
  test('handleResponse returns the correct data for item endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    const request = {
      url: `/v1/acl/role/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = Object.assign({}, payload, {
        Datacenter: dc,
        [META]: {},
        uid: `["${dc}","${id}"]`,
        Policies: createPolicies(payload),
      });
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
});
