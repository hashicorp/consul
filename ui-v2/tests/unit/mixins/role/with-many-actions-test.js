import EmberObject from '@ember/object';
import RoleWithManyActionsMixin from 'consul-ui/mixins/role/with-many-actions';
import { module, test } from 'qunit';

module('Unit | Mixin | role/with many actions');

// Replace this with your real tests.
test('it works', function(assert) {
  let RoleWithManyActionsObject = EmberObject.extend(RoleWithManyActionsMixin);
  let subject = RoleWithManyActionsObject.create();
  assert.ok(subject);
});
