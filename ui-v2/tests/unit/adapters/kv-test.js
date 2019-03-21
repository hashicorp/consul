import { module, skip } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import stubSuper from 'consul-ui/tests/helpers/stub-super';

module('Unit | Adapter | kv', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    assert.ok(adapter);
  });
});
