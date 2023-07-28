import EmberObject from '@ember/object';
import RoleAsManyMixin from 'consul-ui/mixins/role/as-many';
import { module, test } from 'qunit';

module('Unit | Mixin | role/as many', function () {
  // Replace this with your real tests.
  test('it works', function (assert) {
    let RoleAsManyObject = EmberObject.extend(RoleAsManyMixin);
    let subject = RoleAsManyObject.create();
    assert.ok(subject);
  });
});
