import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
} from 'consul-ui/utils/http/consul';
import { createPolicies } from 'consul-ui/tests/helpers/normalizers';

module('Integration | Serializer | role', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'role-name';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`respondForQuery returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:role');
      const request = {
        url: `/v1/acl/roles?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}`,
      };
      return get(request.url).then(function(payload) {
        const expected = payload.map(item =>
          Object.assign({}, item, {
            Datacenter: dc,
            Policies: createPolicies(item),
            Namespace: item.Namespace || undefinedNspace,
            uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.ID}"]`,
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
      const serializer = this.owner.lookup('serializer:role');
      const request = {
        url: `/v1/acl/role/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}`,
      };
      return get(request.url).then(function(payload) {
        const expected = Object.assign({}, payload, {
          Datacenter: dc,
          Policies: createPolicies(payload),
          [META]: {
            [DC.toLowerCase()]: dc,
            [NSPACE.toLowerCase()]: nspace || '',
          },
          Namespace: payload.Namespace || undefinedNspace,
          uid: `["${payload.Namespace || undefinedNspace}","${dc}","${id}"]`,
        });
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
