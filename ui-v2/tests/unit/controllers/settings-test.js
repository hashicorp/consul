import { moduleFor, test } from 'ember-qunit';

moduleFor('controller:settings', 'Unit | Controller | settings', {
  // Specify the other units that are required for this test.
  needs: ['service:settings', 'service:dom', 'service:timeout'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let controller = this.subject();
  assert.ok(controller);
});
