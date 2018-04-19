import EmberObject from '@ember/object';
import WithKeyUtilsMixin from 'consul-ui/mixins/with-key-utils';
import { module, test } from 'qunit';

module('Unit | Mixin | with key utils');

// Replace this with your real tests.
test('it works', function(assert) {
  let WithKeyUtilsObject = EmberObject.extend(WithKeyUtilsMixin);
  let subject = WithKeyUtilsObject.create();
  assert.ok(subject);
});
