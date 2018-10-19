import EmberObject from '@ember/object';
import CreatingRouteMixin from 'consul-ui/mixins/creating-route';
import { module, test } from 'qunit';

module('Unit | Mixin | creating route');

// Replace this with your real tests.
test('it works', function(assert) {
  let CreatingRouteObject = EmberObject.extend(CreatingRouteMixin);
  let subject = CreatingRouteObject.create();
  assert.ok(subject);
});
