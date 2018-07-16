import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import makeAttrable from 'consul-ui/utils/makeAttrable';
module('Integration | Adapter | session | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'session-id';
  test('urlForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const node = 'node-id';
    const expected = `/v1/session/node/${node}?dc=${dc}`;
    const actual = adapter.urlForQuery({
      dc: dc,
      id: node,
    });
    assert.equal(actual, expected);
  });
  test('urlForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const expected = `/v1/session/info/${id}?dc=${dc}`;
    const actual = adapter.urlForQueryRecord({
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("urlForQuery throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    assert.throws(function() {
      adapter.urlForQuery({
        dc: dc,
      });
    });
  });
  test("urlForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    assert.throws(function() {
      adapter.urlForQueryRecord({
        dc: dc,
      });
    });
  });
  test('urlForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const expected = `/v1/session/destroy/${id}?dc=${dc}`;
    const actual = adapter.urlForDeleteRecord(
      id,
      'acl',
      makeAttrable({
        Datacenter: dc,
        ID: id,
      })
    );
    assert.equal(actual, expected);
  });
});
