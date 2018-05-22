import { moduleFor, test } from 'ember-qunit';

moduleFor('service:feedback', 'Unit | Service | feedback', {
  // Specify the other units that are required for this test.
  needs: ['service:logger', 'service:flashMessages'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
