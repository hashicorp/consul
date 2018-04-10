import { moduleFor, skip } from 'ember-qunit';

moduleFor('route:dc/kv/create', 'Unit | Route | dc/kv/create', {
  // Specify the other units that are required for this test.
  needs: ['service:kv', 'service:feedback']
});

skip('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
