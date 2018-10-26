import { moduleFor, test } from 'ember-qunit';

moduleFor('service:repository/node', 'Unit | Service | node', {
  // Specify the other units that are required for this test.
  needs: ['service:repository/coordinate'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
