import { moduleFor, test } from 'ember-qunit';

moduleFor('service:dc', 'Unit | Service | dc', {
  // Specify the other units that are required for this test.
  needs: ['service:settings'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
