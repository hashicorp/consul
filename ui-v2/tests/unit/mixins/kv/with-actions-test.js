import EmberObject from '@ember/object';
import KvWithActionsMixin from 'consul-ui/mixins/kv/with-actions';
import { moduleFor, test } from 'ember-qunit';

moduleFor('mixin:kv/with-actions', 'Unit | Mixin | kv/with actions', {
  // Specify the other units that are required for this test.
  needs: ['mixin:with-feedback'],
  subject: function() {
    const KvWithActionsObject = EmberObject.extend(KvWithActionsMixin);
    this.register('test-container:kv/with-actions-object', KvWithActionsObject);
    // TODO: May need to actually get this from the container
    return KvWithActionsObject;
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
