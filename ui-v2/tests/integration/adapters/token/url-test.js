import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | token | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'policy-id';
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/acl/tokens?dc=${dc}`;
    const actual = adapter.requestForQuery(client.url, {
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('urlForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/acl/token/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
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
  test('requestForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/acl/token?dc=${dc}`;
    const actual = adapter
      .requestForCreateRecord(client.url, {
        Datacenter: dc,
      })
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForUpdateRecord returns the correct url (without Rules it uses the v2 API)', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const expected = `POST /v1/acl/token/${id}?dc=${dc}`;
    const actual = adapter
      .requestForUpdateRecord(client.url, {
        Datacenter: dc,
        AccessorID: id,
      })
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForUpdateRecord returns the correct url (with Rules it uses the v1 API)', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const expected = `POST /v1/acl/update?dc=${dc}`;
    const actual = adapter
      .requestForUpdateRecord(client.url, {
        Rules: 'key {}',
        Datacenter: dc,
        AccessorID: id,
      })
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const client = this.owner.lookup('service:client/http');
    const expected = `DELETE /v1/acl/token/${id}?dc=${dc}`;
    const actual = adapter
      .requestForDeleteRecord(client.url, {
        Datacenter: dc,
        AccessorID: id,
      })
      .split('\n')[0];
    assert.equal(actual, expected);
  });
});
