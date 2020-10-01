import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
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
        assert.equal(actual[0].Namespace, expected[0].Namespace);
        assert.equal(actual[0].Datacenter, expected[0].Datacenter);
        assert.equal(actual[0].uid, expected[0].uid);
      });
    });
    test(`respondForQuery returns the correct data for list endpoint when gateway is set when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:service');
      const gateway = 'gateway';
      const request = {
        url: `/v1/internal/ui/gateway-services-nodes/${gateway}?dc=${dc}${
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
            gateway: gateway,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
