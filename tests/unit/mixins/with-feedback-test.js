import EmberObject from '@ember/object';
import WithFeedbackMixin from 'consul-ui/mixins/with-feedback';
import { module, skip } from 'qunit';

module('Unit | Mixin | with feedback');

// Replace this with your real tests.
skip('it works', function(assert) {
  let WithFeedbackObject = EmberObject.extend(WithFeedbackMixin);
  let subject = WithFeedbackObject.create();
  assert.ok(subject);
});
