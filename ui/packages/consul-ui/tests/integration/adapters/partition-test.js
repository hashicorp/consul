import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Integration | Adapter | partition', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'slug';
  test('requestForQuery returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:partition');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/partition?dc=${dc}`;
    const actual = adapter.requestForQuery(client.url, {
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('requestForQueryRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:partition');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    const expected = `GET /v1/partition/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(request, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:partition');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    assert.throws(function() {
      adapter.requestForQueryRecord(request, {
        dc: dc,
      });
    });
  });
});
