import EmberObject from '@ember/object';
import PolicyWithManyActionsMixin from 'consul-ui/mixins/policy/with-many-actions';
import { module, test } from 'qunit';

module('Unit | Mixin | policy/with many actions');

// Replace this with your real tests.
test('it works', function(assert) {
  let PolicyWithManyActionsObject = EmberObject.extend(PolicyWithManyActionsMixin);
  let subject = PolicyWithManyActionsObject.create();
  assert.ok(subject);
});
