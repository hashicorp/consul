import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | acl', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'token-name';
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/acl/list?dc=${dc}`;
    const actual = adapter.requestForQuery(client.url, {
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('requestForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/acl/info/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test('requestForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/acl/create?dc=${dc}`;
    const actual = adapter
      .requestForCreateRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          ID: id,
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForUpdateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/acl/update?dc=${dc}`;
    const actual = adapter
      .requestForUpdateRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          ID: id,
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/acl/destroy/${id}?dc=${dc}`;
    const actual = adapter
      .requestForDeleteRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          ID: id,
        }
      )
      .split('/n')[0];
    assert.equal(actual, expected);
  });
  test('requestForCloneRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/acl/clone/${id}?dc=${dc}`;
    const actual = adapter
      .requestForCloneRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          ID: id,
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
});
