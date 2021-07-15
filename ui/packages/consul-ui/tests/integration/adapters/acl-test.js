import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | acl', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'token-name';
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    const expected = `GET /v1/acl/list?dc=${dc}`;
    const actual = adapter.requestForQuery(request, {
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('requestForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    const expected = `GET /v1/acl/info/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(request, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.throws(function() {
      adapter.requestForQueryRecord(request, {
        dc: dc,
      });
    });
  });
  test('requestForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    const expected = `PUT /v1/acl/create?dc=${dc}`;
    const actual = adapter
      .requestForCreateRecord(
        request,
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
    const request = client.url.bind(client);
    const expected = `PUT /v1/acl/update?dc=${dc}`;
    const actual = adapter
      .requestForUpdateRecord(
        request,
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
    const request = client.url.bind(client);
    const expected = `PUT /v1/acl/destroy/${id}?dc=${dc}`;
    const actual = adapter
      .requestForDeleteRecord(
        request,
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
    const request = client.url.bind(client);
    const expected = `PUT /v1/acl/clone/${id}?dc=${dc}`;
    const actual = adapter
      .requestForCloneRecord(
        request,
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
