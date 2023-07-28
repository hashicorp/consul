import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | dc/acls/policies/edit', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:dc/acls/policies/edit');
    assert.ok(route);
  });
});
