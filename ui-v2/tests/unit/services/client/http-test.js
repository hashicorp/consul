import { moduleFor, test } from 'ember-qunit';

moduleFor('service:client/http', 'Unit | Service | client/http', {
  // Specify the other units that are required for this test.
  needs: ['service:dom', 'service:settings'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
