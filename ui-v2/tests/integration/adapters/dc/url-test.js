import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | dc | url', function(hooks) {
  setupTest(hooks);
  test('urlForFindAll returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:dc');
    const expected = `/v1/catalog/datacenters`;
    const actual = adapter.urlForFindAll();
    assert.equal(actual, expected);
  });
});
