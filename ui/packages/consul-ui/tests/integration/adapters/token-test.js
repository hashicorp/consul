import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { env } from '../../../env';
const shouldHaveNspace = function(nspace) {
  return typeof nspace !== 'undefined' && env('CONSUL_NSPACES_ENABLED');
};
module('Integration | Adapter | token', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'policy-id';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`requestForQuery returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = client.requestParams.bind(client);
      const expected = `GET /v1/acl/tokens?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter.requestForQuery(request, {
        dc: dc,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForQuery returns the correct url/method when a policy is specified when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = client.requestParams.bind(client);
      const expected = `GET /v1/acl/tokens?policy=${id}&dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter.requestForQuery(request, {
        dc: dc,
        policy: id,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForQuery returns the correct url/method when a role is specified when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = client.requestParams.bind(client);
      const expected = `GET /v1/acl/tokens?role=${id}&dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter.requestForQuery(request, {
        dc: dc,
        role: id,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForQueryRecord returns the correct url/method when nspace is ${nspace}`, async function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = function() {
        return () => client.requestParams.bind(client)(...arguments);
      };
      const expected = `GET /v1/acl/token/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = await adapter.requestForQueryRecord(request, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      actual = actual();
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForCreateRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const expected = `PUT /v1/acl/token?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForCreateRecord(
          request,
          {},
          {
            Datacenter: dc,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test(`requestForUpdateRecord returns the correct url (without Rules it uses the v2 API) when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const expected = `PUT /v1/acl/token/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForUpdateRecord(
          request,
          {},
          {
            Datacenter: dc,
            AccessorID: id,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test(`requestForUpdateRecord returns the correct url (with Rules it uses the v1 API) when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      // As the title of the test says, this one uses the ACL legacy APIs and
      // therefore does not expect a nspace
      const expected = `PUT /v1/acl/update?dc=${dc}`;
      const actual = adapter
        .requestForUpdateRecord(
          request,
          {},
          {
            Rules: 'key {}',
            Datacenter: dc,
            AccessorID: id,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test(`requestForDeleteRecord returns the correct url/method when the nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const expected = `DELETE /v1/acl/token/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForDeleteRecord(
          request,
          {},
          {
            Datacenter: dc,
            AccessorID: id,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test(`requestForCloneRecord returns the correct url when the nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const expected = `PUT /v1/acl/token/${id}/clone?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForCloneRecord(
          request,
          {},
          {
            Datacenter: dc,
            AccessorID: id,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.rejects(
      adapter.requestForQueryRecord(request, {
        dc: dc,
      })
    );
  });
  test('requestForSelf returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    const expected = `GET /v1/acl/token/self?dc=${dc}`;
    const actual = adapter
      .requestForSelf(
        request,
        {},
        {
          dc: dc,
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForSelf sets a token header using a secret', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    const secret = 'sssh';
    const expected = `X-Consul-Token: ${secret}`;
    const actual = adapter
      .requestForSelf(
        request,
        {},
        {
          dc: dc,
          secret: secret,
        }
      )
      .split('\n')[1]
      .trim();
    assert.equal(actual, expected);
  });
});
