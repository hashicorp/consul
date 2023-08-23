import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
  HEADERS_PARTITION as PARTITION,
} from 'consul-ui/utils/http/consul';

module('Integration | Serializer | topology', function(hooks) {
  setupTest(hooks);
  test('respondForQueryRecord returns the correct data for item endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:topology');
    const dc = 'dc-1';
    const id = 'slug';
    const kind = '';
    const nspace = 'default';
    const partition = 'default';
    const request = {
      url: `/v1/internal/ui/service-topology/${id}?dc=${dc}&kind=${kind}`,
    };
    return get(request.url).then(function(payload) {
      const expected = {
        Datacenter: dc,
        [META]: {},
        uid: `["${partition}","${nspace}","${dc}","${id}"]`,
      };
      const actual = serializer.respondForQueryRecord(
        function(cb) {
          const headers = {
            [DC]: dc,
            [NSPACE]: nspace,
            [PARTITION]: partition,
          };
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
          id: id,
          kind: kind,
        }
      );
      assert.equal(actual.Datacenter, expected.Datacenter);
      assert.equal(actual.uid, expected.uid);
    });
  });
});
