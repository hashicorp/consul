import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
  HEADERS_PARTITION as PARTITION,
} from 'consul-ui/utils/http/consul';
module('Integration | Serializer | coordinate', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const nspace = 'default';
  const partition = 'default';
  test('respondForQuery returns the correct data for list endpoint', function (assert) {
    assert.expect(1);
    const serializer = this.owner.lookup('serializer:coordinate');
    const request = {
      url: `/v1/coordinate/nodes?dc=${dc}`,
    };
    return get(request.url).then(function (payload) {
      const expected = payload.map((item) =>
        Object.assign({}, item, {
          Datacenter: dc,
          // TODO: default isn't required here, once we've
          // refactored out our Serializer this can go
          Namespace: nspace,
          Partition: partition,
          uid: `["${partition}","${nspace}","${dc}","${item.Node}"]`,
        })
      );
      const actual = serializer.respondForQuery(
        function (cb) {
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
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
});
