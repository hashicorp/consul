import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | session', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'session-id';
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    const node = 'node-id';
    const expected = `GET /v1/session/node/${node}?dc=${dc}`;
    const actual = adapter.requestForQuery(client.url, {
      dc: dc,
      id: node,
    });
    assert.equal(actual, expected);
  });
  test('requestForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/session/info/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQuery throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQuery(client.url, {
        dc: dc,
      });
    });
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test('urlForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/session/destroy/${id}?dc=${dc}`;
    const actual = adapter
      .requestForDeleteRecord(
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
