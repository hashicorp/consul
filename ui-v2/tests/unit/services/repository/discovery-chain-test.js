import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Repository | discovery-chain', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    let repo = this.owner.lookup('service:repository/discovery-chain');
    assert.ok(repo);
  });
});
