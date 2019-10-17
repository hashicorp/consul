import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';
module('Integration | Serializer | service', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`respondForQuery returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:service');
      const request = {
        url: `/v1/internal/ui/services?dc=${dc}${
          typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
        }`,
      };
      return get(request.url).then(function(payload) {
        const expected = payload.map(item =>
          Object.assign({}, item, {
            Namespace: item.Namespace || undefinedNspace,
            Datacenter: dc,
            uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.Name}"]`,
          })
        );
        const actual = serializer.respondForQuery(
          function(cb) {
            const headers = {};
            const body = payload;
            return cb(headers, body);
          },
          {
            dc: dc,
            ns: nspace,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
    test(`respondForQueryRecord returns the correct data for item endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:service');
      const id = 'service-name';
      const request = {
        url: `/v1/health/service/${id}?dc=${dc}${
          typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
        }`,
      };
      return get(request.url).then(function(payload) {
        const expected = {
          Datacenter: dc,
          [META]: {},
          Namespace: payload[0].Service.Namespace || undefinedNspace,
          uid: `["${payload[0].Service.Namespace || undefinedNspace}","${dc}","${id}"]`,
          Name: id,
          Nodes: payload,
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
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
