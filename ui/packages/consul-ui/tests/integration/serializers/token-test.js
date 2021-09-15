import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
  HEADERS_PARTITION as PARTITION,
} from 'consul-ui/utils/http/consul';

import { createPolicies } from 'consul-ui/tests/helpers/normalizers';

module('Integration | Serializer | token', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'token-name';
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`respondForQuery returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:token');
      const request = {
        url: `/v1/acl/tokens?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
          typeof partition !== 'undefined' ? `&partition=${partition}` : ``
        }`,
      };
      return get(request.url).then(function(payload) {
        const expected = payload.map(item =>
          Object.assign({}, item, {
            Datacenter: dc,
            Namespace: item.Namespace || undefinedNspace,
            Partition: item.Partition || undefinedPartition,
            uid: `["${item.Partition || undefinedPartition}","${item.Namespace ||
              undefinedNspace}","${dc}","${item.AccessorID}"]`,
            Policies: createPolicies(item),
          })
        );
        const actual = serializer.respondForQuery(
          function(cb) {
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
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
    test(`respondForQueryRecord returns the correct data for item endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:token');
      const request = {
        url: `/v1/acl/token/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
          typeof partition !== 'undefined' ? `&partition=${partition}` : ``
        }`,
      };
      return get(request.url).then(function(payload) {
        const expected = Object.assign({}, payload, {
          Datacenter: dc,
          [META]: {
            [DC.toLowerCase()]: dc,
            [NSPACE.toLowerCase()]: nspace || undefinedNspace,
            [PARTITION.toLowerCase()]: partition || undefinedPartition,
          },
          Namespace: payload.Namespace || undefinedNspace,
          Partition: payload.Partition || undefinedPartition,
          uid: `["${payload.Partition || undefinedPartition}","${payload.Namespace ||
            undefinedNspace}","${dc}","${id}"]`,
          Policies: createPolicies(payload),
        });
        const actual = serializer.respondForQueryRecord(
          function(cb) {
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
            partition: partition,
            id: id,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
