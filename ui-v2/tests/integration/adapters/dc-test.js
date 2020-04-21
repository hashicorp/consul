import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | dc', function(hooks) {
  setupTest(hooks);
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:dc');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/catalog/datacenters`;
    const actual = adapter.requestForQuery(client.url);
    assert.equal(actual, expected);
  });
});
