import { moduleFor, test } from 'ember-qunit';

moduleFor('service:nodes', 'Unit | Service | nodes', {
  // Specify the other units that are required for this test.
  needs: ['service:coordinates'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
