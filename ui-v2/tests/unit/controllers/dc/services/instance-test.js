import { moduleFor, test } from 'ember-qunit';

moduleFor('controller:dc/services/instance', 'Unit | Controller | dc/services/instance', {
  // Specify the other units that are required for this test.
  needs: ['service:dom', 'service:flashMessages'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let controller = this.subject();
  assert.ok(controller);
});
