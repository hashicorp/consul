import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
  HEADERS_PARTITION as PARTITION,
} from 'consul-ui/utils/http/consul';
module('Integration | Serializer | kv', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'key-name/here';
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`respondForQuery returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:kv');
      const request = {
        url: `/v1/kv/${id}?keys&dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
          typeof partition !== 'undefined' ? `&partition=${partition}` : ``
        }`,
      };
      return get(request.url).then(function(payload) {
        const expected = payload.map(item =>
          Object.assign(
            {},
            {
              Key: item,
            },
            {
              Datacenter: dc,
              // the payload here is just an array of strings
              // so we reuse the query param
              Namespace: nspace || undefinedNspace,
              Partition: partition || undefinedPartition,
              uid: `["${partition || undefinedPartition}","${nspace ||
                undefinedNspace}","${dc}","${item}"]`,
            }
          )
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
            partition: partition,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
    test(`respondForQueryRecord returns the correct data for item endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:kv');
      const request = {
        url: `/v1/kv/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
          typeof partition !== 'undefined' ? `&partition=${partition}` : ``
        }`,
      };
      return get(request.url).then(function(payload) {
        const expected = Object.assign({}, payload[0], {
          Datacenter: dc,
          [META]: {
            [DC.toLowerCase()]: dc,
            [NSPACE.toLowerCase()]: nspace || undefinedNspace,
            [PARTITION.toLowerCase()]: partition || undefinedPartition,
          },
          Namespace: payload[0].Namespace || undefinedNspace,
          uid: `["${payload[0].Partition || undefinedPartition}","${payload[0].Namespace ||
            undefinedNspace}","${dc}","${id}"]`,
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
            ns: nspace || undefinedNspace,
            partition: partition || undefinedPartition,
            id: id,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
