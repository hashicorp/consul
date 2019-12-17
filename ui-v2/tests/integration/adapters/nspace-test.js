import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

// nspaces aren't categorized by datacenter therefore no dc
module('Integration | Adapter | nspace', function(hooks) {
  setupTest(hooks);
  const id = 'slug';
  test('requestForQuery returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:nspace');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/namespaces`;
    const actual = adapter.requestForQuery(client.url, {});
    assert.equal(actual, expected);
  });
  test('requestForQueryRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:nspace');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/namespace/${id}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:nspace');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {});
    });
  });
});
