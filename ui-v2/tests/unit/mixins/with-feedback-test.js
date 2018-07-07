import EmberObject from '@ember/object';
import WithFeedbackMixin from 'consul-ui/mixins/with-feedback';
import { moduleFor, test } from 'ember-qunit';

moduleFor('mixin:with-feedback', 'Unit | Mixin | with feedback', {
  // Specify the other units that are required for this test.
  needs: ['service:feedback'],
  subject: function() {
    const WithFeedbackObject = EmberObject.extend(WithFeedbackMixin);
    this.register('test-container:with-feedback-object', WithFeedbackObject);
    // TODO: May need to actually get this from the container
    return WithFeedbackObject;
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
