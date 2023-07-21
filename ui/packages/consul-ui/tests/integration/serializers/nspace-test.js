import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_DATACENTER as DC,
  HEADERS_PARTITION as PARTITION,
} from 'consul-ui/utils/http/consul';
// Nspaces don't need any nspace
module('Integration | Serializer | nspace', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedPartition = 'default';
  const partition = 'default';
  test('respondForQuery returns the correct data for list endpoint', function (assert) {
    assert.expect(1);
    const serializer = this.owner.lookup('serializer:nspace');
    const request = {
      url: `/v1/namespaces?dc=${dc}${
        typeof partition !== 'undefined' ? `&partition=${partition}` : ``
      }`,
    };
    return get(request.url).then(function (payload) {
      const expected = payload.map((item) =>
        Object.assign({}, item, {
          Datacenter: dc,
          Partition: item.Partition || undefinedPartition,
          Namespace: '*',
          uid: `["${item.Partition}","*","${dc}","${item.Name}"]`,
        })
      );
      const actual = serializer.respondForQuery(
        function (cb) {
          const headers = {
            [DC]: dc,
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
  test('respondForQueryRecord returns the correct data for item endpoint', function (assert) {
    assert.expect(1);
    const serializer = this.owner.lookup('serializer:nspace');
    const id = 'slug';
    const request = {
      url: `/v1/namespace/${id}?dc=${dc}${
        typeof partition !== 'undefined' ? `&partition=${partition}` : ``
      }`,
    };
    return get(request.url).then(function (payload) {
      // Namespace items don't currently get META attached
      const expected = payload;
      const actual = serializer.respondForQueryRecord(
        function (cb) {
          const headers = {
            [DC]: dc,
            [PARTITION]: partition || undefinedPartition,
          };
          const body = payload;
          return cb(headers, body);
        },
        {
          id: id,
          dc: dc,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
});
