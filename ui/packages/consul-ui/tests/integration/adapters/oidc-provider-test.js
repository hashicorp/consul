import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { env } from '../../../env';
const shouldHaveNspace = function(nspace) {
  return typeof nspace !== 'undefined' && env('CONSUL_NSPACES_ENABLED');
};
module('Integration | Adapter | oidc-provider', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'slug';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test('requestForQuery returns the correct url/method', function(assert) {
      const adapter = this.owner.lookup('adapter:oidc-provider');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/internal/ui/oidc-auth-methods?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter.requestForQuery(client.requestParams.bind(client), {
        dc: dc,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test('requestForQueryRecord returns the correct url/method', function(assert) {
      const adapter = this.owner.lookup('adapter:oidc-provider');
      const client = this.owner.lookup('service:client/http');
      const expected = `POST /v1/acl/oidc/auth-url?dc=${dc}`;
      const actual = adapter
        .requestForQueryRecord(client.url, {
          dc: dc,
          id: id,
          ns: nspace,
        })
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test("requestForQueryRecord throws if you don't specify an id", function(assert) {
      const adapter = this.owner.lookup('adapter:oidc-provider');
      const client = this.owner.lookup('service:client/http');
      assert.throws(function() {
        adapter.requestForQueryRecord(client.url, {
          dc: dc,
        });
      });
    });
    test('requestForAuthorize returns the correct url/method', function(assert) {
      const adapter = this.owner.lookup('adapter:oidc-provider');
      const client = this.owner.lookup('service:client/http');
      const expected = `POST /v1/acl/oidc/callback?dc=${dc}`;
      const actual = adapter
        .requestForAuthorize(client.url, {
          dc: dc,
          id: id,
          code: 'code',
          state: 'state',
          ns: nspace,
        })
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test('requestForLogout returns the correct url/method', function(assert) {
      const adapter = this.owner.lookup('adapter:oidc-provider');
      const client = this.owner.lookup('service:client/http');
      const expected = `POST /v1/acl/logout`;
      const actual = adapter
        .requestForLogout(client.url, {
          id: id,
        })
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
  });
});
