import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Repository | gateway', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    const repo = this.owner.lookup('service:repository/gateway');
    assert.ok(repo);
  });
});
