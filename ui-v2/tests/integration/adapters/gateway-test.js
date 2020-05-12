import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Integration | Adapter | gateway', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'slug';
  test('requestForQueryRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:gateway');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/internal/ui/gateway-services-nodes/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:gateway');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
});
