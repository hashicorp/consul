import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Service | auth-method', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    let service = this.owner.lookup('service:repository/auth-method');
    assert.ok(service);
  });
});
