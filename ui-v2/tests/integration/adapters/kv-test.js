import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | kv', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'key-name/here';
  test('requestForQuery returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/kv/${id}?keys&dc=${dc}`;
    const actual = adapter.requestForQuery(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test('requestForQueryRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/kv/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test("requestForQuery throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQuery(client.url, {
        dc: dc,
      });
    });
  });
  test('requestForCreateRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/kv/${id}?dc=${dc}`;
    const actual = adapter
      .requestForCreateRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          Key: id,
          Value: '',
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForUpdateRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/kv/${id}?dc=${dc}`;
    const actual = adapter
      .requestForUpdateRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          Key: id,
          Value: '',
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForDeleteRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    const expected = `DELETE /v1/kv/${id}?dc=${dc}`;
    const actual = adapter.requestForDeleteRecord(
      client.url,
      {},
      {
        Datacenter: dc,
        Key: id,
      }
    );
    assert.equal(actual, expected);
  });
  test('requestForDeleteRecord returns the correct url/method for folders', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    const folder = `${id}/`;
    const expected = `DELETE /v1/kv/${folder}?dc=${dc}&recurse`;
    const actual = adapter.requestForDeleteRecord(
      client.url,
      {},
      {
        Datacenter: dc,
        Key: folder,
      }
    );
    assert.equal(actual, expected);
  });
});
