import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import makeAttrable from 'consul-ui/utils/makeAttrable';
module('Integration | Adapter | kv | url', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'key-name/here';
  test('slugFromURL returns the correct slug', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const url = `/v1/kv/${id}?dc=${dc}`;
    const expected = id;
    const actual = adapter.slugFromURL(new URL(url, 'http://localhost'));
    assert.equal(actual, expected);
  });
  test('urlForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const expected = `/v1/kv/${id}?keys&dc=${dc}`;
    const actual = adapter.urlForQuery({
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test('urlForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const expected = `/v1/kv/${id}?dc=${dc}`;
    const actual = adapter.urlForQueryRecord({
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("urlForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    assert.throws(function() {
      adapter.urlForQueryRecord({
        dc: dc,
      });
    });
  });
  test("urlForQuery throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    assert.throws(function() {
      adapter.urlForQuery({
        dc: dc,
      });
    });
  });
  test('urlForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const expected = `/v1/kv/${id}?dc=${dc}`;
    const actual = adapter.urlForCreateRecord(
      'kv',
      makeAttrable({
        Datacenter: dc,
        Key: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForUpdateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const expected = `/v1/kv/${id}?dc=${dc}`;
    const actual = adapter.urlForUpdateRecord(
      id,
      'kv',
      makeAttrable({
        Datacenter: dc,
        Key: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForDeleteRecord returns the correct url for non-folders', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const expected = `/v1/kv/${id}?dc=${dc}`;
    const actual = adapter.urlForDeleteRecord(
      id,
      'kv',
      makeAttrable({
        Datacenter: dc,
        Key: id,
      })
    );
    assert.equal(actual, expected);
  });
  test('urlForDeleteRecord returns the correct url for folders', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const folder = `${id}/`;
    const expected = `/v1/kv/${folder}?dc=${dc}&recurse`;
    const actual = adapter.urlForDeleteRecord(
      folder,
      'kv',
      makeAttrable({
        Datacenter: dc,
        Key: folder,
      })
    );
    assert.equal(actual, expected);
  });
});
