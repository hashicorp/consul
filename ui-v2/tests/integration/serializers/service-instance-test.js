import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
} from 'consul-ui/utils/http/consul';
module('Integration | Serializer | service-instance', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`respondForQueryRecord returns the correct data for item endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:service-instance');
      const id = 'service-name';
      const node = 'node-0';
      const request = {
        url: `/v1/health/service/${id}?dc=${dc}${
          typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
        }`,
      };
      return get(request.url).then(function(payload) {
        payload[0].Node.Node = node;
        payload[0].Service.ID = id;
        const expected = {
          ...payload[0],
          Datacenter: dc,
          [META]: {
            [DC.toLowerCase()]: dc,
            [NSPACE.toLowerCase()]: payload[0].Service.Namespace || undefinedNspace,
          },
          Namespace: payload[0].Service.Namespace || undefinedNspace,
          uid: `["${payload[0].Service.Namespace || undefinedNspace}","${dc}","${node}","${id}"]`,
        };
        const actual = serializer.respondForQueryRecord(
          function(cb) {
            const headers = {};
            const body = payload;
            return cb(headers, body);
          },
          {
            dc: dc,
            ns: nspace,
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
