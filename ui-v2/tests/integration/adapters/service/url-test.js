import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | service | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'service-name';
  test('urlForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:service');
    const expected = `/v1/internal/ui/services?dc=${dc}`;
    const actual = adapter.urlForQuery({
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('urlForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:service');
    const expected = `/v1/health/service/${id}?dc=${dc}`;
    const actual = adapter.urlForQueryRecord({
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("urlForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:service');
    assert.throws(function() {
      adapter.urlForQueryRecord({
        dc: dc,
      });
    });
  });
});
