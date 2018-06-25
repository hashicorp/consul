import EmberObject from '@ember/object';
import IntentionWithActionsMixin from 'consul-ui/mixins/intention/with-actions';
import { moduleFor, test } from 'ember-qunit';

moduleFor('mixin:intention/with-actions', 'Unit | Mixin | intention/with actions', {
  // Specify the other units that are required for this test.
  needs: ['service:feedback'],
  subject: function() {
    const IntentionWithActionsObject = EmberObject.extend(IntentionWithActionsMixin);
    this.register('test-container:intention/with-actions-object', IntentionWithActionsObject);
    // TODO: May need to actually get this from the container
    return IntentionWithActionsObject;
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
