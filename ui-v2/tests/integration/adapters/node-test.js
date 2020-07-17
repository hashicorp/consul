import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | node', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'node-name';
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:node');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/internal/ui/nodes?dc=${dc}`;
    const actual = adapter.requestForQuery(client.requestParams.bind(client), {
      dc: dc,
    });
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test('requestForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:node');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/internal/ui/node/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.requestParams.bind(client), {
      dc: dc,
      id: id,
    });
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:node');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test('requestForQueryLeader returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:node');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/status/leader?dc=${dc}`;
    const actual = adapter.requestForQueryLeader(client.url, {
      dc: dc,
    });
    assert.equal(actual, expected);
  });
});
