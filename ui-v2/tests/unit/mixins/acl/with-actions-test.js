import EmberObject from '@ember/object';
import AclWithActionsMixin from 'consul-ui/mixins/acl/with-actions';
import { module, test } from 'qunit';

module('Unit | Mixin | acl/with actions');

// Replace this with your real tests.
test('it works', function(assert) {
  let AclWithActionsObject = EmberObject.extend(AclWithActionsMixin);
  let subject = AclWithActionsObject.create();
  assert.ok(subject);
});
