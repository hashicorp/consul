import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | dc/intentions/index', function(hooks) {
  setupTest(hooks);

  test('it exists', function(assert) {
    let route = this.owner.lookup('route:dc/intentions/index');
    assert.ok(route);
  });
});
