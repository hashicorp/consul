import EmberObject from '@ember/object';
import WithFeedbackMixin from 'consul-ui/mixins/with-feedback';
import { module, test } from 'qunit';

module('Unit | Mixin | with feedback');

// Replace this with your real tests.
test('it works', function(assert) {
  let WithFeedbackObject = EmberObject.extend(WithFeedbackMixin);
  let subject = WithFeedbackObject.create();
  assert.ok(subject);
});
