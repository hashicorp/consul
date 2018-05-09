import { moduleFor, test } from 'ember-qunit';

moduleFor('controller:dc/kv/create', 'Unit | Controller | dc/kv/create', {
  // Specify the other units that are required for this test.
  needs: ['service:btoa'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let controller = this.subject();
  assert.ok(controller);
});
