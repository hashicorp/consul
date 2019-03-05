import EmberObject from '@ember/object';
import WithResizingMixin from 'consul-ui/mixins/with-resizing';
import { module, test } from 'qunit';

module('Unit | Mixin | with resizing');

// Replace this with your real tests.
test('it works', function(assert) {
  let WithResizingObject = EmberObject.extend(WithResizingMixin);
  let subject = WithResizingObject.create();
  assert.ok(subject);
});
