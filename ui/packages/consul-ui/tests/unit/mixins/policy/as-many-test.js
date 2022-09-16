import EmberObject from '@ember/object';
import PolicyAsManyMixin from 'consul-ui/mixins/policy/as-many';
import { module, test } from 'qunit';

module('Unit | Mixin | policy/as many', function () {
  // Replace this with your real tests.
  test('it works', function (assert) {
    let PolicyAsManyObject = EmberObject.extend(PolicyAsManyMixin);
    let subject = PolicyAsManyObject.create();
    assert.ok(subject);
  });
});
