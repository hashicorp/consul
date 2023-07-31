import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Service | clipboard/local storage', function (hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function (assert) {
    let service = this.owner.lookup('service:clipboard/local-storage');
    assert.ok(service);
  });
});
