import { moduleFor, test } from 'ember-qunit';

moduleFor('service:repository/policy', 'Unit | Service | policy', {
  // Specify the other units that are required for this test.
  needs: ['service:store'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
