import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import makeAttrable from 'consul-ui/utils/makeAttrable';
module('Integration | Adapter | role | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'role-name';
  test('urlForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    const expected = `/v1/acl/roles?dc=${dc}`;
    const actual = adapter.urlForQuery({
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('urlForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    const expected = `/v1/acl/role/${id}?dc=${dc}`;
    const actual = adapter.urlForQueryRecord({
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("urlForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    assert.throws(function() {
      adapter.urlForQueryRecord({
        dc: dc,
      });
    });
  });
  test('urlForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    const expected = `/v1/acl/role?dc=${dc}`;
    const actual = adapter.urlForCreateRecord(
      'role',
      makeAttrable({
        Datacenter: dc,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForUpdateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    const expected = `/v1/acl/role/${id}?dc=${dc}`;
    const actual = adapter.urlForUpdateRecord(
      id,
      'role',
      makeAttrable({
        Datacenter: dc,
        ID: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:role');
    const expected = `/v1/acl/role/${id}?dc=${dc}`;
    const actual = adapter.urlForDeleteRecord(
      id,
      'role',
      makeAttrable({
        Datacenter: dc,
        ID: id,
      })
    );
    assert.equal(actual, expected);
  });
});
