import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Repository | <%= dasherizedModuleName %>', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function(assert) {
    const repo = this.owner.lookup('service:repository/<%= dasherizedModuleName %>');
    assert.ok(repo);
  });
});
