import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | token', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'policy-id';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`requestForQuery returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/acl/tokens?dc=${dc}`;
      let actual = adapter.requestForQuery(client.url, {
        dc: dc,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${typeof nspace !== 'undefined' ? `ns=${nspace}` : ``}`);
    });
    test(`requestForQuery returns the correct url/method when a policy is specified when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/acl/tokens?policy=${id}&dc=${dc}`;
      let actual = adapter.requestForQuery(client.url, {
        dc: dc,
        policy: id,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${typeof nspace !== 'undefined' ? `ns=${nspace}` : ``}`);
    });
    test(`requestForQuery returns the correct url/method when a role is specified when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/acl/tokens?role=${id}&dc=${dc}`;
      let actual = adapter.requestForQuery(client.url, {
        dc: dc,
        role: id,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${typeof nspace !== 'undefined' ? `ns=${nspace}` : ``}`);
    });
    test(`requestForQueryRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/acl/token/${id}?dc=${dc}`;
      let actual = adapter.requestForQueryRecord(client.url, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${typeof nspace !== 'undefined' ? `ns=${nspace}` : ``}`);
    });
    test(`requestForCreateRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:token');
      const client = this.owner.lookup('service:client/http');
      const expected = `PUT /v1/acl/token?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForCreateRecord(
          client.url,
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
      const expected = `PUT /v1/acl/token/${id}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForUpdateRecord(
          client.url,
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
      const expected = `PUT /v1/acl/update?dc=${dc}`;
      const actual = adapter
        .requestForUpdateRecord(
          client.url,
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
      const expected = `DELETE /v1/acl/token/${id}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForDeleteRecord(
          client.url,
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
      const expected = `PUT /v1/acl/token/${id}/clone?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForCloneRecord(
          client.url,
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
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test('requestForSelf returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/acl/token/self?dc=${dc}`;
    const actual = adapter
      .requestForSelf(
        client.url,
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
    const secret = 'sssh';
    const expected = `X-Consul-Token: ${secret}`;
    const actual = adapter
      .requestForSelf(
        client.url,
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
