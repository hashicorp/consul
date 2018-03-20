import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Adapter | kv', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    assert.ok(adapter);
  });
  test('handleResponse returns a Kv-like object when the request is a createRecord', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    // unflake, this is also going through _super and
    // using urlForCreateRecord and makeAttrable so not the unit
    const expected = 'key/name';
    const actual = adapter.handleResponse(200, {}, true, {
      method: 'GET',
      url: `/v1/kv/${expected}`,
    });
    assert.deepEqual(actual, {
      Key: expected,
      Datacenter: '',
    });
  });
});
