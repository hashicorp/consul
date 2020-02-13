import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | coordinate', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:coordinate');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/coordinate/nodes?dc=${dc}`;
    const actual = adapter.requestForQuery(client.url, {
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('requestForQuery returns the correct body', function(assert) {
    const adapter = this.owner.lookup('adapter:coordinate');
    const client = this.owner.lookup('service:client/http');
    const expected = {
      index: 1,
    };
    const actual = adapter.requestForQuery(client.body, {
      dc: dc,
      index: 1,
    });
    assert.deepEqual(actual, expected);
  });
});
