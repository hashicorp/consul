import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
  HEADERS_PARTITION as PARTITION,
} from 'consul-ui/utils/http/consul';
module('Integration | Serializer | service-instance', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`respondForQueryRecord returns the correct data for item endpoint when nspace is ${nspace}`, function (assert) {
      assert.expect(1);

      const serializer = this.owner.lookup('serializer:service-instance');
      const id = 'service-name';
      const node = 'node-0';
      const request = {
        url: `/v1/health/service/${id}?dc=${dc}${
          typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
        }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`,
      };
      return get(request.url).then(function (payload) {
        payload[0].Node.Node = node;
        payload[0].Service.ID = id;
        const expected = {
          ...payload[0],
          Datacenter: dc,
          [META]: {
            [DC.toLowerCase()]: dc,
            [NSPACE.toLowerCase()]: nspace || undefinedNspace,
            [PARTITION.toLowerCase()]: partition || undefinedPartition,
          },
          Namespace: payload[0].Service.Namespace || undefinedNspace,
          Partition: payload[0].Service.Partition || undefinedPartition,
          uid: `["${payload[0].Service.Partition || undefinedPartition}","${
            payload[0].Service.Namespace || undefinedNspace
          }","${dc}","${node}","${id}"]`,
        };
        const actual = serializer.respondForQueryRecord(
          function (cb) {
            const headers = {
              [DC]: dc,
              [NSPACE]: nspace || undefinedNspace,
              [PARTITION]: partition || undefinedPartition,
            };
            const body = payload;
            return cb(headers, body);
          },
          {
            dc: dc,
            ns: nspace,
            partition: partition || undefinedPartition,
            id: id,
            node: node,
            serviceId: id,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
