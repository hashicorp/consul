import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
} from 'consul-ui/utils/http/consul';

module('Integration | Serializer | gateway', function(hooks) {
  setupTest(hooks);
  test('respondForQueryRecord returns the correct data for item endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:gateway');
    const dc = 'dc-1';
    const id = 'slug';
    const nspace = 'default';
    const request = {
      url: `/v1/internal/ui/gateway-services-nodes/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = {
        Datacenter: dc,
        [META]: {
          [DC.toLowerCase()]: dc,
          [NSPACE.toLowerCase()]: nspace,
        },
        uid: `["${nspace}","${dc}","${id}"]`,
        Name: id,
        Namespace: nspace,
        Services: payload,
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
