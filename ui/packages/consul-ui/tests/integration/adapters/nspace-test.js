import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

// nspaces aren't categorized by datacenter therefore no dc
module('Integration | Adapter | nspace', function(hooks) {
  setupTest(hooks);
  const id = 'slug';
  test('requestForQuery returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:nspace');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    const expected = `GET /v1/namespaces`;
    const actual = adapter.requestForQuery(request, {});
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test('requestForQueryRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:nspace');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    const expected = `GET /v1/namespace/${id}`;
    const actual = adapter.requestForQueryRecord(request, {
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:nspace');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.throws(function() {
      adapter.requestForQueryRecord(request, {});
    });
  });
});
