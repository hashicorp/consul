import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/kv/index', 'Unit | Route | dc/kv/index', {
  // Specify the other units that are required for this test.
  needs: ['service:kv', 'service:feedback', 'service:flashMessages'],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
