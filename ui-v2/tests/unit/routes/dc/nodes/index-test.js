import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/nodes/index', 'Unit | Route | dc/nodes/index', {
  // Specify the other units that are required for this test.
  needs: ['service:nodes']
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
