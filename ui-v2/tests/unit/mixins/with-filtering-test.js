import EmberObject from '@ember/object';
import WithFilteringMixin from 'consul-ui/mixins/with-filtering';
import { module, test } from 'qunit';

module('Unit | Mixin | with filtering', function() {
  // Replace this with your real tests.
  test('it works', function(assert) {
    let WithFilteringObject = EmberObject.extend(WithFilteringMixin);
    let subject = WithFilteringObject.create();
    assert.ok(subject);
  });
});
