import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import makeAttrable from 'consul-ui/utils/makeAttrable';
module('Integration | Adapter | acl | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'token-name';
  test('urlForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const expected = `/v1/acl/list?dc=${dc}`;
    const actual = adapter.urlForQuery({
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('urlForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const expected = `/v1/acl/info/${id}?dc=${dc}`;
    const actual = adapter.urlForQueryRecord({
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("urlForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    assert.throws(function() {
      adapter.urlForQueryRecord({
        dc: dc,
      });
    });
  });
  test('urlForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const expected = `/v1/acl/create?dc=${dc}`;
    const actual = adapter.urlForCreateRecord(
      'acl',
      makeAttrable({
        Datacenter: dc,
        ID: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForUpdateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const expected = `/v1/acl/update?dc=${dc}`;
    const actual = adapter.urlForUpdateRecord(
      id,
      'acl',
      makeAttrable({
        Datacenter: dc,
        ID: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const expected = `/v1/acl/destroy/${id}?dc=${dc}`;
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
  test('urlForCloneRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:acl');
    const expected = `/v1/acl/clone/${id}?dc=${dc}`;
    const actual = adapter.urlForCloneRecord(
      'acl',
      makeAttrable({
        Datacenter: dc,
        ID: id,
      })
    );
    assert.equal(actual, expected);
  });
});
