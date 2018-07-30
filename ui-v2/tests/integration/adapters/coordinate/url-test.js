import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | coordinate | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  test('urlForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:coordinate');
    const expected = `/v1/coordinate/nodes?dc=${dc}`;
    const actual = adapter.urlForQuery({
      dc: dc,
    });
    assert.equal(actual, expected);
  });
});
