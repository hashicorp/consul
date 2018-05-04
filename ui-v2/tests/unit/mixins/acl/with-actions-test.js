import EmberObject from '@ember/object';
import AclWithActionsMixin from 'consul-ui/mixins/acl/with-actions';
import { moduleFor, test } from 'ember-qunit';

moduleFor('mixin:acl/with-actions', 'Unit | Mixin | acl/with actions', {
  // Specify the other units that are required for this test.
  needs: ['mixin:with-feedback'],
  subject: function() {
    const AclWithActionsObject = EmberObject.extend(AclWithActionsMixin);
    this.register('test-container:acl/with-actions-object', AclWithActionsObject);
    // TODO: May need to actually get this from the container
    return AclWithActionsObject;
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
