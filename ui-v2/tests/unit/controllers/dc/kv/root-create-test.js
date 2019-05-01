import { moduleFor, test } from 'ember-qunit';

moduleFor('controller:dc/kv/root-create', 'Unit | Controller | dc/kv/root-create', {
  // Specify the other units that are required for this test.
  needs: [
    'service:btoa',
    'service:dom',
    'service:form',
    'service:repository/role',
    'service:repository/policy',
  ],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let controller = this.subject();
  assert.ok(controller);
});
