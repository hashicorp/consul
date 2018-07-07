import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/kv/root-create', 'Unit | Route | dc/kv/root create', {
  // Specify the other units that are required for this test.
  needs: ['service:kv', 'service:feedback', 'service:logger', 'service:flashMessages'],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
