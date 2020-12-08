import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
} from 'consul-ui/utils/http/consul';

module('Integration | Serializer | oidc-provider', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`respondForQuery returns the correct data for list endpoint when the nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:oidc-provider');
      const request = {
        url: `/v1/internal/ui/oidc-auth-methods?dc=${dc}`,
      };
      return get(request.url).then(function(payload) {
        const expected = payload.map(item =>
          Object.assign({}, item, {
            Datacenter: dc,
            Namespace: item.Namespace || undefinedNspace,
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
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
    test(`respondForQueryRecord returns the correct data for item endpoint when the nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:oidc-provider');
      const dc = 'dc-1';
      const id = 'slug';
      const request = {
        url: `/v1/acl/oidc/auth-url?dc=${dc}`,
      };
      return get(request.url).then(function(payload) {
        const expected = Object.assign({}, payload, {
          Name: id,
          Datacenter: dc,
          [META]: {
            [DC.toLowerCase()]: dc,
            [NSPACE.toLowerCase()]: payload.Namespace || undefinedNspace,
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
            id: id,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
