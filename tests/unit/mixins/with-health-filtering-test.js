import EmberObject from '@ember/object';
import WithHealthFilteringMixin from 'consul-ui/mixins/with-health-filtering';
import { module, test } from 'qunit';

module('Unit | Mixin | with health filtering');

// Replace this with your real tests.
test('it works', function(assert) {
  let WithHealthFilteringObject = EmberObject.extend(WithHealthFilteringMixin);
  let subject = WithHealthFilteringObject.create();
  assert.ok(subject);
});
