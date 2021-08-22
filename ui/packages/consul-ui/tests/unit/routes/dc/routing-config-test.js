import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | dc/routing-config', function(hooks) {
  setupTest(hooks);

  test('it exists', function(assert) {
    let route = this.owner.lookup('route:dc/routing-config');
    assert.ok(route);
  });
});
