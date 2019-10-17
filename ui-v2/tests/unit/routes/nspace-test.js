import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | nspace', function(hooks) {
  setupTest(hooks);

  test('it exists', function(assert) {
    let route = this.owner.lookup('route:nspace');
    assert.ok(route);
  });
});
