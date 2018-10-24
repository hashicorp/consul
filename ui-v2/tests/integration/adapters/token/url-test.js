import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import makeAttrable from 'consul-ui/utils/makeAttrable';
module('Integration | Adapter | token | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'policy-id';
  test('urlForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const expected = `/v1/acl/tokens?dc=${dc}`;
    const actual = adapter.urlForQuery({
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('urlForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const expected = `/v1/acl/token/${id}?dc=${dc}`;
    const actual = adapter.urlForQueryRecord({
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("urlForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    assert.throws(function() {
      adapter.urlForQueryRecord({
        dc: dc,
      });
    });
  });
  test('urlForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const expected = `/v1/acl/token?dc=${dc}`;
    const actual = adapter.urlForCreateRecord(
      'token',
      makeAttrable({
        Datacenter: dc,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForUpdateRecord returns the correct url (without Rules it uses the v2 API)', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const expected = `/v1/acl/token/${id}?dc=${dc}`;
    const actual = adapter.urlForUpdateRecord(
      id,
      'token',
      makeAttrable({
        Datacenter: dc,
        AccessorID: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForUpdateRecord returns the correct url (with Rules it uses the v1 API)', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const expected = `/v1/acl/update?dc=${dc}`;
    const actual = adapter.urlForUpdateRecord(
      id,
      'token',
      makeAttrable({
        Rules: 'key {}',
        Datacenter: dc,
        AccessorID: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:token');
    const expected = `/v1/acl/token/${id}?dc=${dc}`;
    const actual = adapter.urlForDeleteRecord(
      id,
      'token',
      makeAttrable({
        Datacenter: dc,
        AccessorID: id,
      })
    );
    assert.equal(actual, expected);
  });
});
