import EmberObject from '@ember/object';
import KvActionsMixin from 'consul-ui/mixins/kv/with-actions';
import { module, test } from 'qunit';

module('Unit | Mixin | kv/with-actions');

// Replace this with your real tests.
test('it works', function(assert) {
  let KvActionsObject = EmberObject.extend(KvActionsMixin);
  let subject = KvActionsObject.create();
  assert.ok(subject);
});
