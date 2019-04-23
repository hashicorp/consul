import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';

import { createPolicies } from 'consul-ui/tests/helpers/normalizers';

module('Integration | Adapter | token | response', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'token-name';
  test('handleResponse returns the correct data for list endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const request = {
      url: `/v1/acl/tokens?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload.map(item =>
        Object.assign({}, item, {
          Datacenter: dc,
          uid: `["${dc}","${item.AccessorID}"]`,
          Policies: createPolicies(item),
        })
      );
      const actual = adapter.handleResponse(200, {}, payload, request);
      assert.deepEqual(actual, expected);
    });
  });
  test('handleResponse returns the correct data for item endpoint', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const request = {
      url: `/v1/acl/token/${id}?dc=${dc}`,
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
