import { moduleFor, test } from 'ember-qunit';

moduleFor('controller:dc/acls/policies/create', 'Unit | Controller | dc/acls/policies/create', {
  // Specify the other units that are required for this test.
  needs: ['service:form', 'service:dom', 'service:repository/role', 'service:repository/policy'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let controller = this.subject();
  assert.ok(controller);
});
