import { moduleFor, test } from 'ember-qunit';

moduleFor('service:form', 'Unit | Service | form', {
  // Specify the other units that are required for this test.
  needs: ['service:repository/role', 'service:repository/policy'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
