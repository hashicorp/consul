import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import makeAttrable from 'consul-ui/utils/makeAttrable';
module('Integration | Adapter | policy | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'policy-name';
  test('urlForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const expected = `/v1/acl/policies?dc=${dc}`;
    const actual = adapter.urlForQuery({
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('urlForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const expected = `/v1/acl/policy/${id}?dc=${dc}`;
    const actual = adapter.urlForQueryRecord({
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("urlForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    assert.throws(function() {
      adapter.urlForQueryRecord({
        dc: dc,
      });
    });
  });
  test('urlForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const expected = `/v1/acl/policy?dc=${dc}`;
    const actual = adapter.urlForCreateRecord(
      'policy',
      makeAttrable({
        Datacenter: dc,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForUpdateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const expected = `/v1/acl/policy/${id}?dc=${dc}`;
    const actual = adapter.urlForUpdateRecord(
      id,
      'policy',
      makeAttrable({
        Datacenter: dc,
        ID: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const expected = `/v1/acl/policy/${id}?dc=${dc}`;
    const actual = adapter.urlForDeleteRecord(
      id,
      'policy',
      makeAttrable({
        Datacenter: dc,
        ID: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForTranslateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const expected = `/v1/acl/policy/translate`;
    const actual = adapter.urlForTranslateRecord('translate');
    assert.equal(actual, expected);
  });
});
