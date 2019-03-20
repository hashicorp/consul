import EmberObject from '@ember/object';
import RoleWithActionsMixin from 'consul-ui/mixins/role/with-actions';
import { module, test } from 'qunit';

module('Unit | Mixin | role/with actions');

// Replace this with your real tests.
test('it works', function(assert) {
  let RoleWithActionsObject = EmberObject.extend(RoleWithActionsMixin);
  let subject = RoleWithActionsObject.create();
  assert.ok(subject);
});
