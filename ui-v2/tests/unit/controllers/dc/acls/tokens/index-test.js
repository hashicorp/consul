import { moduleFor, test } from 'ember-qunit';

moduleFor('controller:dc/acls/tokens/index', 'Unit | Controller | dc/acls/tokens/index', {
  // Specify the other units that are required for this test.
  needs: ['service:search', 'service:dom', 'service:repository/role', 'service:repository/policy'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let controller = this.subject();
  assert.ok(controller);
});
