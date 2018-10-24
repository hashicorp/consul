import EmberObject from '@ember/object';
import WithListenersMixin from 'consul-ui/mixins/with-listeners';
import { module, test } from 'qunit';

module('Unit | Mixin | with listeners');

// Replace this with your real tests.
test('it works', function(assert) {
  let WithListenersObject = EmberObject.extend(WithListenersMixin);
  let subject = WithListenersObject.create();
  assert.ok(subject);
});
