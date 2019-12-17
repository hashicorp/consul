import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | role', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'role-name';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`requestForQuery returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:role');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/acl/roles?dc=${dc}`;
      let actual = adapter.requestForQuery(client.url, {
        dc: dc,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${typeof nspace !== 'undefined' ? `ns=${nspace}` : ``}`);
    });
    test(`requestForQueryRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:role');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/acl/role/${id}?dc=${dc}`;
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
      const adapter = this.owner.lookup('adapter:role');
      const client = this.owner.lookup('service:client/http');
      const expected = `PUT /v1/acl/role?dc=${dc}${
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
    test(`requestForUpdateRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:role');
      const client = this.owner.lookup('service:client/http');
      const expected = `PUT /v1/acl/role/${id}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForUpdateRecord(
          client.url,
          {},
          {
            Datacenter: dc,
            ID: id,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test(`requestForDeleteRecord returns the correct url/method when the nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:role');
      const client = this.owner.lookup('service:client/http');
      const expected = `DELETE /v1/acl/role/${id}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForDeleteRecord(
          client.url,
          {},
          {
            Datacenter: dc,
            ID: id,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
});
