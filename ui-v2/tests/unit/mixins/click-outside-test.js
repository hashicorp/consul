import EmberObject from '@ember/object';
import ClickOutsideMixin from 'consul-ui/mixins/click-outside';
import { module, test } from 'qunit';

module('Unit | Mixin | click outside');

// Replace this with your real tests.
test('it works', function(assert) {
  let ClickOutsideObject = EmberObject.extend(ClickOutsideMixin);
  let subject = ClickOutsideObject.create();
  assert.ok(subject);
});
